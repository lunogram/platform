package v1

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/http/auth/verifiers"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	httpjson "github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/mailer"
	"github.com/lunogram/platform/internal/password"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

// credentialBodyLimit bounds the request body the login throttle peeks at. It is
// generous for a JSON object holding an address and a password, and small
// enough that peeking cannot be turned into a memory sink.
const credentialBodyLimit = 64 << 10

// passwordAuth holds what the local credential flows need beyond the login
// callback: the mailer that carries every token, and the deployment's answer to
// who may create an account.
type passwordAuth struct {
	enabled      bool
	registration string
	mail         *mailer.Dispatcher
	renderer     *mailer.Renderer
}

func (c *AuthController) initPasswordAuth(cfg config.Node) error {
	c.password.registration = strings.ToLower(strings.TrimSpace(cfg.Auth.Password.Registration))
	c.password.enabled = cfg.Auth.Enabled(verifiers.PasswordDriver)

	if !c.password.enabled {
		return nil
	}

	// Unset falls back to the documented default rather than failing, so a
	// caller that builds the configuration in code -- rather than from the
	// environment, where envDefault supplies it -- still gets the safe answer.
	if c.password.registration == "" {
		c.password.registration = config.RegistrationInviteOnly
	}

	switch c.password.registration {
	case config.RegistrationOpen, config.RegistrationInviteOnly, config.RegistrationDisabled:
	default:
		return errors.New("AUTH_PASSWORD_REGISTRATION must be one of open, invite_only or disabled")
	}

	renderer, err := mailer.NewRenderer("Lunogram", cfg.PublicBaseURL())
	if err != nil {
		return err
	}
	c.password.renderer = renderer

	transport, err := mailer.New(cfg.Mail, c.logger.Named("mailer"))
	if err != nil {
		return err
	}
	c.password.mail = mailer.NewDispatcher(transport, c.logger.Named("mailer"), cfg.Mail.Timeout)

	return nil
}

// RegisterWithPassword creates an account for an address and mails it a
// confirmation link.
//
// It answers 204 for a brand-new address, for an address that already has an
// account, for an address that registration is closed to, and for a send that
// failed -- the response says only "we have dealt with your request". Anything
// finer-grained turns the endpoint into a way to test which addresses hold
// accounts here. The address's owner learns what actually happened by email,
// which is the one channel that reaches only them.
func (c *AuthController) RegisterWithPassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !c.password.enabled || c.password.registration == config.RegistrationDisabled {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("registration is not available on this deployment")))
		return
	}

	var body oapi.RegisterWithPasswordJSONRequestBody
	if err := httpjson.Decode(r.Body, &body); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	email := normaliseEmail(body.Email)
	if email == "" {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("an email address is required")))
		return
	}

	// The password policy is enforced before anything else, and is the one thing
	// this endpoint reports honestly: whether a password is long enough is a
	// fact about the password, not about the address.
	if err := password.Validate(body.Password, email); err != nil {
		oapi.WriteProblem(w, passwordPolicyProblem(err))
		return
	}

	// Registration causes mail to be sent to an address the caller chose, so it
	// is bounded per source. Without this the endpoint is a way to bury a
	// third party's inbox behind our sending reputation.
	budgets := map[budget]string{registerSourceUse: c.throttle.sourceKey(r)}
	if tripped, retryAfter := c.throttle.exceeded(ctx, budgets); tripped {
		writeTooManyRequests(w, retryAfter)
		return
	}
	c.throttle.spend(ctx, budgets)

	// Hashing happens before the address is looked up, so the expensive step
	// runs identically whether or not an account exists. Doing it afterwards
	// would make "this address is taken" measurably faster to detect.
	hash, err := password.Hash(body.Password)
	if err != nil {
		c.logger.Error("failed to hash a password", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	c.register(ctx, email, hash, body)
	w.WriteHeader(http.StatusNoContent)
}

// register does the work behind [AuthController.RegisterWithPassword]. It
// reports nothing: every outcome is a 204, and the outcome that matters to the
// address's owner is delivered by mail.
func (c *AuthController) register(ctx context.Context, email, hash string, body oapi.RegisterWithPasswordJSONRequestBody) {
	logger := c.logger.With(zap.String("flow", "register"))

	existing, err := c.mgmt.ResolveAdminByEmail(ctx, email)
	switch {
	case err == nil:
		// Telling the caller would hand them an account list one address at a
		// time. Telling the owner, in their own inbox, costs an attacker nothing
		// they did not already have.
		logger.Info("registration attempted on an address that already has an account",
			zap.String("admin_id", existing.ID.String()))
		c.sendAccountExists(ctx, existing, email)
		return
	case errors.Is(err, management.ErrAmbiguousEmail):
		// The address is contested by a duplicate the reconciliation
		// quarantined, so nobody can say whose it is. Creating a third account
		// on it would make that worse.
		logger.Error("registration attempted on a contested address")
		return
	case !errors.Is(err, sql.ErrNoRows):
		logger.Error("failed to look up an address for registration", zap.Error(err))
		return
	}

	allowed, err := c.registrationAllowed(ctx, email)
	if err != nil {
		logger.Error("failed to decide whether registration is open", zap.Error(err))
		return
	}
	if !allowed {
		// Logged, not answered: an invite-only deployment that returned a
		// distinguishable status would let anybody test which addresses have
		// been invited.
		logger.Info("registration refused: this deployment is invite only and the address has no pending invite")
		return
	}

	identity := &auth.VerifiedIdentity{
		Issuer:   management.PasswordIssuer,
		Provider: management.IdentityProviderPassword,
		Email:    email,
		// A brand-new local account has proved nothing about its address yet.
		// It may sign in, but until the confirmation link is followed it must
		// not be linkable to any other identity by email.
		EmailVerified: false,
		FirstName:     trimmedName(body.FirstName),
		LastName:      trimmedName(body.LastName),
	}

	adminID, _, err := c.exchanger.ProvisionAdmin(ctx, identity,
		// The subject of a local identity is the admin's own id, which does not
		// exist until the transaction has created them.
		func(adminID uuid.UUID) (string, string, error) { return adminID.String(), hash, nil })
	if err != nil {
		// A conflict here means the address was registered between the lookup
		// above and this write. It is still a 204 to the caller; there is simply
		// nothing further to do.
		logger.Error("failed to provision a registered admin", zap.Error(err))
		return
	}

	logger.Info("registered a new admin", zap.String("admin_id", adminID.String()))
	c.sendVerification(ctx, adminID, email)
}

// registrationAllowed answers whether this deployment will create an account for
// an address.
//
// invite_only additionally admits the very first account: nobody could have
// invited it, and a deployment whose safe default made it impossible to create
// the first admin would just be broken. The window closes the moment one admin
// exists.
func (c *AuthController) registrationAllowed(ctx context.Context, email string) (bool, error) {
	if c.password.registration == config.RegistrationOpen {
		return true, nil
	}

	exists, err := c.mgmt.AdminsExist(ctx)
	if err != nil {
		return false, err
	}
	if !exists {
		return true, nil
	}

	if _, err := c.mgmt.GetPendingInviteOrganization(ctx, email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// VerifyEmail redeems a confirmation token.
func (c *AuthController) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !c.password.enabled {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("password authentication is not available on this deployment")))
		return
	}

	var body oapi.VerifyEmailJSONRequestBody
	if err := httpjson.Decode(r.Body, &body); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	adminID, err := c.mgmt.ConsumeAdminActionToken(ctx, management.ActionTokenEmailVerification, management.HashActionToken(body.Token))
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			c.logger.Error("failed to redeem a verification token", zap.Error(err))
		}
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("this confirmation link is no longer valid")))
		return
	}

	if err := c.markVerified(ctx, adminID); err != nil {
		c.logger.Error("failed to confirm an email address", zap.String("admin_id", adminID.String()), zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	c.logger.Info("confirmed an email address", zap.String("admin_id", adminID.String()))
	w.WriteHeader(http.StatusNoContent)
}

// RequestPasswordReset mails a single-use reset link to an address that has a
// local password.
//
// It always answers 204. The whole point of the flow is that somebody who has
// lost access can use it, which means the caller is not authenticated, which
// means any distinguishable response is an account oracle.
func (c *AuthController) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !c.password.enabled {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("password authentication is not available on this deployment")))
		return
	}

	var body oapi.RequestPasswordResetJSONRequestBody
	if err := httpjson.Decode(r.Body, &body); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	email := normaliseEmail(body.Email)
	if email == "" {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("an email address is required")))
		return
	}

	// Both budgets are keyed on what the caller submitted, never on what was
	// found, so a 429 says nothing about whether the address has an account.
	budgets := map[budget]string{
		resetAccountBudget: accountKey(email),
		resetSourceBudget:  c.throttle.sourceKey(r),
	}
	if tripped, retryAfter := c.throttle.exceeded(ctx, budgets); tripped {
		writeTooManyRequests(w, retryAfter)
		return
	}
	c.throttle.spend(ctx, budgets)

	c.requestReset(ctx, email)
	w.WriteHeader(http.StatusNoContent)
}

func (c *AuthController) requestReset(ctx context.Context, email string) {
	logger := c.logger.With(zap.String("flow", "password_reset"))

	admin, err := c.mgmt.ResolveAdminByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) && !errors.Is(err, management.ErrAmbiguousEmail) {
			logger.Error("failed to look up an address for a password reset", zap.Error(err))
		}
		return
	}

	// An account that signs in through an upstream has no password here to
	// reset, and minting one on their behalf would create a second way into an
	// account whose owner never asked for one.
	if _, err := c.mgmt.GetPasswordIdentity(ctx, admin.ID); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			logger.Error("failed to look up a password identity", zap.Error(err))
		}
		return
	}

	token, hash, err := management.NewActionToken()
	if err != nil {
		logger.Error("failed to generate a reset token", zap.Error(err))
		return
	}

	if err := c.mgmt.CreateAdminActionToken(ctx, admin.ID, management.ActionTokenPasswordReset, hash, management.PasswordResetTTL); err != nil {
		logger.Error("failed to record a reset token", zap.Error(err))
		return
	}

	logger.Info("sent a password reset link", zap.String("admin_id", admin.ID.String()))
	c.password.mail.Dispatch(c.password.renderer.PasswordReset(admin.Email, token, management.PasswordResetTTL))
}

// ConfirmPasswordReset sets a new password from a reset token and ends every
// session the account holds.
//
// Revoking is not optional. A reset is what somebody reaches for when their
// account has been taken over, and leaving the attacker's sessions alive would
// make the remedy useless.
func (c *AuthController) ConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !c.password.enabled {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("password authentication is not available on this deployment")))
		return
	}

	var body oapi.ConfirmPasswordResetJSONRequestBody
	if err := httpjson.Decode(r.Body, &body); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	// The token is redeemed before the new password is validated, so a rejected
	// password does not burn the link the caller is holding.
	adminID, identity, err := c.resolveResetTarget(ctx, body.Token)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	admin, err := c.mgmt.GetAdmin(ctx, adminID)
	if err != nil {
		c.logger.Error("failed to load an admin for a password reset", zap.String("admin_id", adminID.String()), zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	if err := password.Validate(body.Password, admin.Email); err != nil {
		oapi.WriteProblem(w, passwordPolicyProblem(err))
		return
	}

	hash, err := password.Hash(body.Password)
	if err != nil {
		c.logger.Error("failed to hash a password", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	if err := c.mgmt.SetAdminIdentitySecret(ctx, identity.ID, hash); err != nil {
		c.logger.Error("failed to store a reset password", zap.String("admin_id", adminID.String()), zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	// Following the link proves control of the mailbox, which is exactly what
	// the confirmation flow proves. An account that reset its password through
	// its inbox has therefore verified its address, and without this an admin
	// who lost their confirmation mail would have no way to ever confirm.
	if !identity.EmailVerified {
		if err := c.mgmt.MarkAdminIdentityEmailVerified(ctx, identity.ID); err != nil {
			c.logger.Error("failed to record a verified address after a reset", zap.Error(err))
		}
	}

	c.invalidateResetTokens(ctx, adminID)

	if err := c.mgmt.RevokeAdminSessionsForAdmin(ctx, adminID); err != nil {
		c.logger.Error("failed to revoke sessions after a password reset", zap.String("admin_id", adminID.String()), zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("the password was changed but existing sessions could not be ended; contact your administrator")))
		return
	}

	c.logger.Info("completed a password reset", zap.String("admin_id", adminID.String()))
	c.password.mail.Dispatch(c.password.renderer.PasswordChanged(admin.Email))

	w.WriteHeader(http.StatusNoContent)
}

// resolveResetTarget redeems a reset token and returns the identity it
// authorises changing.
func (c *AuthController) resolveResetTarget(ctx context.Context, token string) (uuid.UUID, *management.AdminIdentity, error) {
	invalid := problem.ErrBadRequest(problem.Describe("this reset link is no longer valid"))

	adminID, err := c.mgmt.ConsumeAdminActionToken(ctx, management.ActionTokenPasswordReset, management.HashActionToken(token))
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			c.logger.Error("failed to redeem a reset token", zap.Error(err))
		}
		return uuid.Nil, nil, invalid
	}

	identity, err := c.mgmt.GetPasswordIdentity(ctx, adminID)
	if err != nil {
		// The password identity was removed after the link was sent, so there is
		// nothing left for the token to act on.
		if !errors.Is(err, sql.ErrNoRows) {
			c.logger.Error("failed to load a password identity for a reset", zap.Error(err))
		}
		return uuid.Nil, nil, invalid
	}

	return adminID, identity, nil
}

// ChangePassword sets a new password for the authenticated caller.
//
// The current password is required even though the caller is already
// authenticated: a session that was borrowed -- a shared machine, a stolen
// cookie -- must not be enough to lock the account's owner out of it.
func (c *AuthController) ChangePassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !c.password.enabled {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("password authentication is not available on this deployment")))
		return
	}

	actor := rbac.FromContext(ctx)
	adminID, err := adminActorID(actor)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var body oapi.ChangePasswordJSONRequestBody
	if err := httpjson.Decode(r.Body, &body); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := c.logger.With(zap.String("admin_id", adminID.String()), zap.String("flow", "change_password"))

	admin, err := c.mgmt.GetAdmin(ctx, adminID)
	if err != nil {
		logger.Error("failed to load an admin for a password change", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	identity, err := c.mgmt.GetPasswordIdentity(ctx, adminID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("this account does not sign in with a password")))
			return
		}
		logger.Error("failed to load a password identity", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	if identity.SecretHash == nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("this account does not sign in with a password")))
		return
	}

	match, _, err := password.Verify(*identity.SecretHash, body.CurrentPassword)
	if err != nil {
		logger.Error("stored password hash could not be read", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}
	if !match {
		// Throttled on the same budget as a failed sign-in: this endpoint
		// verifies a password too, so leaving it unbounded would just move the
		// guessing here.
		c.throttle.spend(ctx, map[budget]string{
			loginAccountBudget: accountKey(admin.Email),
			loginSourceBudget:  c.throttle.sourceKey(r),
		})
		logger.Warn("password change rejected: the current password is wrong")
		oapi.WriteProblem(w, problem.ErrForbidden(problem.Describe("the current password is not correct")))
		return
	}

	if err := password.Validate(body.Password, admin.Email); err != nil {
		oapi.WriteProblem(w, passwordPolicyProblem(err))
		return
	}

	hash, err := password.Hash(body.Password)
	if err != nil {
		logger.Error("failed to hash a password", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	if err := c.mgmt.SetAdminIdentitySecret(ctx, identity.ID, hash); err != nil {
		logger.Error("failed to store a changed password", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	c.invalidateResetTokens(ctx, adminID)

	// Every session but this one goes: changing a password is how somebody
	// evicts whoever else is signed in. Ending the caller's own session too
	// would punish them for doing it, which is how people learn not to.
	keep := uuid.Nil
	if claims, ok := c.currentSession(r); ok {
		keep = claims.SessionID
	}
	if err := c.mgmt.RevokeAdminSessionsForAdminExcept(ctx, adminID, keep); err != nil {
		logger.Error("failed to revoke other sessions after a password change", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("the password was changed but other sessions could not be ended; contact your administrator")))
		return
	}

	logger.Info("changed a password")
	c.password.mail.Dispatch(c.password.renderer.PasswordChanged(admin.Email))

	w.WriteHeader(http.StatusNoContent)
}

// markVerified records a confirmed address on the admin's password identity.
func (c *AuthController) markVerified(ctx context.Context, adminID uuid.UUID) error {
	identity, err := c.mgmt.GetPasswordIdentity(ctx, adminID)
	if err != nil {
		return err
	}
	return c.mgmt.MarkAdminIdentityEmailVerified(ctx, identity.ID)
}

// invalidateResetTokens burns the reset links outstanding for an admin. A link
// already in flight is a way back into the account, and closing that door is
// most of the point of changing the password.
func (c *AuthController) invalidateResetTokens(ctx context.Context, adminID uuid.UUID) {
	if err := c.mgmt.InvalidateAdminActionTokens(ctx, adminID, management.ActionTokenPasswordReset); err != nil {
		c.logger.Error("failed to invalidate outstanding reset tokens",
			zap.String("admin_id", adminID.String()), zap.Error(err))
	}
}

func (c *AuthController) sendVerification(ctx context.Context, adminID uuid.UUID, email string) {
	token, hash, err := management.NewActionToken()
	if err != nil {
		c.logger.Error("failed to generate a verification token", zap.Error(err))
		return
	}

	if err := c.mgmt.CreateAdminActionToken(ctx, adminID, management.ActionTokenEmailVerification, hash, management.EmailVerificationTTL); err != nil {
		c.logger.Error("failed to record a verification token", zap.Error(err))
		return
	}

	c.password.mail.Dispatch(c.password.renderer.VerifyEmail(email, token))
}

// sendAccountExists tells the owner of an already-registered address that
// somebody tried to register it, and offers them a way back in if it was them.
//
// The link is a real reset token, which is safe: it goes only to the address
// that already owns the account, and it is subject to the same expiry and
// single use as any other.
func (c *AuthController) sendAccountExists(ctx context.Context, admin *management.Admin, email string) {
	if _, err := c.mgmt.GetPasswordIdentity(ctx, admin.ID); err != nil {
		// The account signs in through an upstream, so there is no password to
		// reset and no useful link to offer. Staying silent is better than
		// mailing a dead end.
		return
	}

	token, hash, err := management.NewActionToken()
	if err != nil {
		c.logger.Error("failed to generate a reset token", zap.Error(err))
		return
	}

	if err := c.mgmt.CreateAdminActionToken(ctx, admin.ID, management.ActionTokenPasswordReset, hash, management.PasswordResetTTL); err != nil {
		c.logger.Error("failed to record a reset token", zap.Error(err))
		return
	}

	c.password.mail.Dispatch(c.password.renderer.AccountExists(email, token))
}

// passwordPolicyProblem turns a policy failure into a 400 the console can show.
// These are facts about the submitted password and reveal nothing about the
// account, so unlike everything else on these endpoints they are reported.
func passwordPolicyProblem(err error) error {
	switch {
	case errors.Is(err, password.ErrTooShort):
		return problem.ErrBadRequest(problem.Describe("your password must be at least 12 characters long"))
	case errors.Is(err, password.ErrTooLong):
		return problem.ErrBadRequest(problem.Describe("your password is too long"))
	case errors.Is(err, password.ErrSimilar):
		return problem.ErrBadRequest(problem.Describe("your password must not be based on your email address"))
	case errors.Is(err, password.ErrCommon):
		return problem.ErrBadRequest(problem.Describe("this password is too easily guessed, please choose another"))
	default:
		return problem.ErrBadRequest(problem.Describe("this password cannot be used"))
	}
}

func normaliseEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func trimmedName(name *string) *string {
	if name == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*name)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

// peekCredentialEmail reads the address out of a login body and puts the body
// back, so the verifier still sees exactly what the caller sent.
//
// The throttle has to key on the account before the credential is proved, and
// the verifier is the thing that decodes the body -- reading it here and
// restoring it is what lets both happen without the verifier learning anything
// about throttling.
func peekCredentialEmail(r *http.Request) (string, error) {
	if r.Body == nil {
		return "", nil
	}

	buffered, err := io.ReadAll(io.LimitReader(r.Body, credentialBodyLimit))
	r.Body = io.NopCloser(bytes.NewReader(buffered))
	if err != nil {
		return "", err
	}

	var credentials struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(buffered, &credentials); err != nil {
		// Not JSON we recognise. The verifier will reject it in a moment; the
		// per-source budget still applies.
		return "", nil
	}

	return normaliseEmail(credentials.Email), nil
}
