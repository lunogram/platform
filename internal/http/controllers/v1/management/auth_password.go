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

func (c *AuthController) initPasswordAuth(cfg config.Node, mail *mailer.Dispatcher, renderer *mailer.Renderer) error {
	c.password.registration = strings.ToLower(strings.TrimSpace(cfg.Auth.Basic.Registration))
	// Taken from the built verifiers rather than from the configured names. The
	// registry normalises case and whitespace as it builds, so reading the raw
	// slice here could disable registration, reset and the mail setup for a
	// driver that is nonetheless live and advertised.
	c.password.enabled = c.Verifier(verifiers.BasicDriver) != nil

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
		return errors.New("AUTH_BASIC_REGISTRATION must be one of open, invite_only or disabled")
	}

	// A deployment offering password logins with nowhere to send mail cannot
	// confirm an address or reset a password, so it is refused here rather than
	// discovered by the first person who tries to register. The mailer itself is
	// built once by the caller and shared, because invites go out through the
	// same channel.
	if err := mailer.RequireConfigured(cfg.Mail); err != nil {
		return err
	}
	if mail == nil || renderer == nil {
		return errors.New("password auth requires a mailer")
	}

	c.password.mail = mail
	c.password.renderer = renderer

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

	// Registration causes mail to be sent to an address the caller chose, so it
	// is bounded per source. Without this the endpoint is a way to bury a
	// third party's inbox behind our sending reputation.
	budgets := map[budget]string{registerSourceUse: c.throttle.sourceKey(r)}
	if tripped, retryAfter := c.throttle.claim(ctx, budgets); tripped {
		writeTooManyRequests(w, retryAfter)
		return
	}

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
		Issuer:   management.LocalIssuer,
		Provider: management.IdentityProviderBasic,
		Email:    email,
		// A brand-new local account has proved nothing about its address yet.
		// It may sign in, but until the confirmation link is followed it must
		// not be linkable to any other identity by email.
		EmailVerified: false,
		FirstName:     trimmedName(body.FirstName),
		LastName:      trimmedName(body.LastName),
	}

	adminID, _, err := c.exchanger.ProvisionAdmin(ctx, identity, auth.Provisioning{
		// The subject of a local identity is the admin's own id, which does not
		// exist until the transaction has created them.
		Credential: func(adminID uuid.UUID) (string, string, error) { return adminID.String(), hash, nil },
		// Re-asked inside the transaction that does the writing. The check
		// above is the fast path and the place the reason gets logged; this is
		// the one that decides, because only it is atomic with the insert.
		Admit: c.admitRegistration(email),
	})
	if err != nil {
		if errors.Is(err, errRegistrationClosed) {
			logger.Info("registration refused: another registration claimed the first account first")
			return
		}
		// A conflict here means the address was registered between the lookup
		// above and this write. It is still a 204 to the caller; there is simply
		// nothing further to do.
		logger.Error("failed to provision a registered admin", zap.Error(err))
		return
	}

	logger.Info("registered a new admin", zap.String("admin_id", adminID.String()))
}

// errRegistrationClosed aborts a provisioning transaction whose admission check
// said no. It never reaches the caller: registration answers 204 either way.
var errRegistrationClosed = errors.New("registration is not open to this address")

// registrationAllowed answers whether this deployment will create an account for
// an address.
//
// This is the fast path, and the place the reason is logged. It is NOT the
// decision: [AuthController.admitRegistration] re-asks the same question inside
// the transaction that creates the admin, which is the only place the answer
// can be atomic with the write.
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

// admitRegistration re-decides, inside the provisioning transaction, whether an
// address may have an account created for it.
//
// The bootstrap allowance is what forces this to be transactional. "No admin
// exists yet, so this one may register" is a read that two simultaneous
// registrations both pass, and a deployment meant to admit exactly one founder
// would get two, each owning an organization. The advisory lock is held by the
// transaction, so the loser sees the winner's admin and is refused.
func (c *AuthController) admitRegistration(email string) func(context.Context, *management.State) error {
	return func(ctx context.Context, tx *management.State) error {
		if c.password.registration == config.RegistrationOpen {
			return nil
		}

		if err := tx.LockRegistrationBootstrap(ctx); err != nil {
			return err
		}

		exists, err := tx.AdminsExist(ctx)
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}

		if _, err := tx.GetPendingInviteOrganization(ctx, email); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errRegistrationClosed
			}
			return err
		}

		return nil
	}
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

	adminID, err := c.commitVerification(ctx, body.Token)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			c.logger.Error("failed to confirm an email address", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("this confirmation link is no longer valid")))
		return
	}

	c.logger.Info("confirmed an email address", zap.String("admin_id", adminID.String()))
	c.admitInvitee(ctx, adminID)

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
	if tripped, retryAfter := c.throttle.claim(ctx, budgets); tripped {
		writeTooManyRequests(w, retryAfter)
		return
	}

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
	if _, err := c.mgmt.GetLocalIdentity(ctx, admin.ID); err != nil {
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

	if err := c.mgmt.CreateAdminActionToken(ctx, admin.ID, management.ActionTokenPasswordReset, admin.Email, hash, management.PasswordResetTTL); err != nil {
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

	hash, err := password.Hash(body.Password)
	if err != nil {
		c.logger.Error("failed to hash a password", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	// The credential, the outstanding links and the sessions commit together. A
	// reset is the remedy for a compromised account, so "the password changed
	// but the attacker's sessions are still live" is not a state this flow may
	// leave behind — and it is exactly what a failure between three separate
	// writes used to produce.
	if err := c.commitCredentialChange(ctx, adminID, identity.ID, hash, uuid.Nil); err != nil {
		c.logger.Error("failed to complete a password reset", zap.String("admin_id", adminID.String()), zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("the password could not be reset; nothing was changed")))
		return
	}

	// Following the link proves control of the mailbox, which is exactly what
	// the confirmation flow proves. An account that reset its password through
	// its inbox has therefore verified its address, and without this an admin
	// who lost their confirmation mail would have no way to ever confirm.
	//
	// It is also the moment a pending invite may finally be honoured, for the
	// same reason: the address stopped being a claim and became proof.
	if !identity.EmailVerified {
		if err := c.mgmt.MarkAdminIdentityEmailVerified(ctx, identity.ID); err != nil {
			c.logger.Error("failed to record a verified address after a reset", zap.Error(err))
		} else {
			c.admitInvitee(ctx, adminID)
		}
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

	identity, err := c.mgmt.GetLocalIdentity(ctx, adminID)
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

	identity, err := c.mgmt.GetLocalIdentity(ctx, adminID)
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

	// Verifying a password here is the same work a sign-in does, so it is bounded
	// by the same budgets -- and bounded BEFORE the comparison, not only
	// recorded after it. Spending without checking counts every wrong guess and
	// refuses none of them, which leaves this endpoint accepting unlimited
	// guesses at the current password through a stolen session.
	budgets := map[budget]string{
		loginAccountBudget: accountKey(admin.Email),
		loginSourceBudget:  c.throttle.sourceKey(r),
	}
	if tripped, retryAfter := c.throttle.exceeded(ctx, budgets); tripped {
		logger.Warn("password change refused: the sign-in failure budget is spent")
		writeTooManyRequests(w, retryAfter)
		return
	}

	match, err := password.Verify(*identity.SecretHash, body.CurrentPassword)
	if err != nil {
		logger.Error("stored password hash could not be read", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}
	if !match {
		c.throttle.spend(ctx, budgets)
		logger.Warn("password change rejected: the current password is wrong")
		oapi.WriteProblem(w, problem.ErrForbidden(problem.Describe("the current password is not correct")))
		return
	}

	hash, err := password.Hash(body.Password)
	if err != nil {
		logger.Error("failed to hash a password", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	// Every session but this one goes: changing a password is how somebody
	// evicts whoever else is signed in. Ending the caller's own session too
	// would punish them for doing it, which is how people learn not to.
	keep := uuid.Nil
	if claims, ok := c.currentSession(r); ok {
		keep = claims.SessionID
	}

	if err := c.commitCredentialChange(ctx, adminID, identity.ID, hash, keep); err != nil {
		logger.Error("failed to change a password", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("the password could not be changed; nothing was changed")))
		return
	}

	logger.Info("changed a password")
	c.password.mail.Dispatch(c.password.renderer.PasswordChanged(admin.Email))

	w.WriteHeader(http.StatusNoContent)
}

// markVerified records a confirmed address on the admin's password identity.
// commitVerification redeems a confirmation link and records the address as
// proved, as one transaction.
//
// Separately, the token was spent first: a failure loading or updating the
// identity afterwards left the address unverified and the only link that could
// prove it already consumed, with no way to retry. A link is single-use because
// it must not be replayable, not because a transient database error should burn
// it.
func (c *AuthController) commitVerification(ctx context.Context, token string) (uuid.UUID, error) {
	tx, err := c.db.BeginTxx(ctx, nil)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	store := management.NewState(tx)

	adminID, err := store.ConsumeAdminActionToken(ctx, management.ActionTokenEmailVerification, management.HashActionToken(token))
	if err != nil {
		return uuid.Nil, err
	}

	identity, err := store.GetLocalIdentity(ctx, adminID)
	if err != nil {
		return uuid.Nil, err
	}
	if err := store.MarkAdminIdentityEmailVerified(ctx, identity.ID); err != nil {
		return uuid.Nil, err
	}

	if err := tx.Commit(); err != nil {
		return uuid.Nil, err
	}
	return adminID, nil
}

// admitInvitee grants the organization membership a pending invite implies, now
// that the admin has proved the address it was sent to.
//
// It runs after the address is confirmed, never before: until then the address
// is only what somebody typed into a registration form, and honouring an invite
// on it would let anyone who can guess an invited address walk into that
// organization. Failure is logged and dropped -- the confirmation itself
// succeeded, and the invite stays pending for the ordinary accept flow.
func (c *AuthController) admitInvitee(ctx context.Context, adminID uuid.UUID) {
	admin, err := c.mgmt.GetAdmin(ctx, adminID)
	if err != nil {
		c.logger.Error("failed to load an admin to honour their invite", zap.Error(err))
		return
	}

	err = c.exchanger.AdmitInvitee(ctx, adminID, admin.Email)
	switch {
	case err == nil, errors.Is(err, auth.ErrAlreadyMember), errors.Is(err, sql.ErrNoRows):
		// Nothing pending, or nothing to add. Both are ordinary.
	default:
		c.logger.Error("failed to honour a pending invite after verification",
			zap.String("admin_id", adminID.String()), zap.Error(err))
	}
}

// commitCredentialChange stores a new secret, burns the reset links still
// outstanding and ends sessions, as one transaction.
//
// The three were separate writes, and the order they ran in was the problem: the
// hash landed first, so a failure in either of the others left the password
// changed while the links that could undo it and the sessions it was meant to
// evict were still live. That is precisely the guarantee both flows advertise,
// so all three commit or none of them do.
//
// keep is the session to spare — uuid.Nil ends every one of them, which is what
// a reset wants.
//
// Cache invalidation happens after the commit rather than inside it. A store
// built over a transaction has no cache attached, and invalidating early would
// be wrong regardless: a rollback would leave a live session missing from the
// cache.
func (c *AuthController) commitCredentialChange(ctx context.Context, adminID, identityID uuid.UUID, hash string, keep uuid.UUID) error {
	tx, err := c.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	store := management.NewState(tx)

	if err := store.SetAdminIdentitySecret(ctx, identityID, hash); err != nil {
		return err
	}
	if err := store.InvalidateAdminActionTokens(ctx, adminID, management.ActionTokenPasswordReset); err != nil {
		return err
	}
	revoked, err := store.RevokeAdminSessionsForAdminExcept(ctx, adminID, keep)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	c.mgmt.InvalidateAdminSessionCache(ctx, revoked)
	return nil
}

// sendAccountExists tells the owner of an already-registered address that
// somebody tried to register it, and offers them a way back in if it was them.
//
// The link is a real reset token, which is safe: it goes only to the address
// that already owns the account, and it is subject to the same expiry and
// single use as any other.
func (c *AuthController) sendAccountExists(ctx context.Context, admin *management.Admin, email string) {
	if _, err := c.mgmt.GetLocalIdentity(ctx, admin.ID); err != nil {
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

	if err := c.mgmt.CreateAdminActionToken(ctx, admin.ID, management.ActionTokenPasswordReset, admin.Email, hash, management.PasswordResetTTL); err != nil {
		c.logger.Error("failed to record a reset token", zap.Error(err))
		return
	}

	c.password.mail.Dispatch(c.password.renderer.AccountExists(email, token, management.PasswordResetTTL))
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
