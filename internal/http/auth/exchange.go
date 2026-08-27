package auth

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/rbac/access"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

// VerifiedIdentity is what a [Verifier] produces: proof of who the caller is
// according to an upstream, and nothing else.
//
// A verifier proves a credential and STOPS. It does not find or create admins,
// does not create organizations, does not write RBAC tuples, does not mint
// tokens and does not touch the response -- it is not handed a ResponseWriter,
// by construction. Everything downstream of the proof belongs to the
// [Exchanger], so there is exactly one place where an identity turns into
// access.
type VerifiedIdentity struct {
	// Issuer and Subject are the identity key, taken from the VERIFIED token.
	// Subject is opaque: it is never parsed, split, or interpreted as anything
	// but a string.
	Issuer  string
	Subject string
	// Provider is descriptive (which kind of upstream proved this). It is not
	// part of the identity key.
	Provider string

	Email string
	// EmailVerified gates identity linking by email. Linking on an unverified
	// address turns any IdP that lets a user self-assert an email into an
	// account-takeover primitive: claim the address, log in, inherit the
	// account.
	EmailVerified bool

	// Profile fields populate a NEW admin only. They never overwrite an existing
	// one: the admin owns their own profile, and an upstream rename must not be
	// able to silently rewrite it.
	FirstName *string
	LastName  *string
	ImageURL  *string

	// Actor is non-nil when the upstream session is itself impersonated.
	Actor *VerifiedActor
	// ExpiresAt is the upstream session's expiry. It is required when Actor is
	// set, because an impersonated session may never outlive the session that
	// authorised it.
	ExpiresAt time.Time
}

// VerifiedActor identifies who is impersonating. The subject is the upstream's,
// and usually maps to no admin of ours.
type VerifiedActor struct {
	Subject string
}

// Verifier proves a credential presented at the auth callback. Implementations
// live in internal/http/auth/verifiers.
type Verifier interface {
	// Driver is the identifier the callback route is keyed on.
	Driver() string

	// Verify extracts and proves the credential carried by the request. It
	// returns a [VerifiedIdentity] on success and an error otherwise; it must
	// not write to the database or the response.
	Verify(ctx context.Context, r *http.Request) (*VerifiedIdentity, error)
}

// WebhookVerifier is implemented separately from [Verifier] so a driver without
// webhooks (basic) does not have to carry a method whose only job is to say
// "not supported".
type WebhookVerifier interface {
	Webhook(ctx context.Context, r *http.Request) error
}

// OrgResolver decides which organization a brand-new admin joins, and with what
// role. It exists to isolate the "create a Default Organization" reflex that
// every provider used to perform inline: Phase 2 swaps in a resolver that maps
// issuer -> sso_connection -> that customer's organization, and this is the only
// implementation that has to change.
type OrgResolver interface {
	// Resolve runs inside the provisioning transaction so the organization it
	// creates commits or rolls back together with the admin.
	Resolve(ctx context.Context, tx *management.State, identity *VerifiedIdentity) (organizationID uuid.UUID, role string, err error)
}

// DefaultOrgResolver gives every new admin a fresh organization of their own,
// which they own.
type DefaultOrgResolver struct{}

func (DefaultOrgResolver) Resolve(ctx context.Context, tx *management.State, _ *VerifiedIdentity) (uuid.UUID, string, error) {
	organizationID, err := tx.CreateOrganization(ctx, "Default Organization")
	if err != nil {
		return uuid.Nil, "", err
	}
	return organizationID, rbac.OrganizationOwner, nil
}

// InviteOrgResolver puts an admin whose address was already invited into the
// organization that invited them, and falls back to [DefaultOrgResolver] for
// everybody else.
//
// Without it, being invited and then signing up produces an organization of
// your own that nobody asked for, and the invite you were sent points somewhere
// you are not a member of. The invite itself is left pending: accepting it is
// what grants the project role, and that flow already exists -- this only
// decides where the account lands.
//
// The lookup runs on the address the credential carries, verified or not, which
// is safe because it grants nothing: base membership of an organization that
// deliberately invited this address, with the project role still gated behind
// the existing accept flow.
type InviteOrgResolver struct{}

func (InviteOrgResolver) Resolve(ctx context.Context, tx *management.State, identity *VerifiedIdentity) (uuid.UUID, string, error) {
	organizationID, err := tx.GetPendingInviteOrganization(ctx, identity.Email)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return DefaultOrgResolver{}.Resolve(ctx, tx, identity)
	case err != nil:
		return uuid.Nil, "", err
	}

	return organizationID, rbac.OrganizationMember, nil
}

// ExchangeResult is a freshly minted console session.
type ExchangeResult struct {
	Token     string
	ExpiresAt time.Time
	Session   *management.AdminSession
}

// Exchanger turns a [VerifiedIdentity] into a Lunogram console session: it
// resolves or links the admin, provisions membership when the admin is new,
// records an admin_sessions row, mints the token and sets the cookie.
type Exchanger struct {
	db     *sqlx.DB
	mgmt   *management.State
	engine *rbac.Engine
	signer *ConsoleSigner
	orgs   OrgResolver
	logger *zap.Logger

	// legacyAdoption allows an identity to claim an admin whose identity row
	// still carries the sentinel issuer the dropped external_id column was
	// backfilled under. See [Exchanger.adoptLegacyIdentity] for why that is
	// safe and why it is temporary.
	legacyAdoption bool
}

func NewExchanger(
	db *sqlx.DB,
	mgmt *management.State,
	engine *rbac.Engine,
	signer *ConsoleSigner,
	orgs OrgResolver,
	logger *zap.Logger,
	legacyAdoption bool,
) *Exchanger {
	if orgs == nil {
		orgs = InviteOrgResolver{}
	}
	return &Exchanger{
		db:             db,
		mgmt:           mgmt,
		engine:         engine,
		signer:         signer,
		orgs:           orgs,
		logger:         logger,
		legacyAdoption: legacyAdoption,
	}
}

// Exchange completes a login. When w is non-nil the console session cookie is
// set on it; passing nil mints the session without touching the response.
//
// A login is an event, so it always opens its own session.
func (e *Exchanger) Exchange(ctx context.Context, w http.ResponseWriter, r *http.Request, identity *VerifiedIdentity) (*ExchangeResult, error) {
	if e.signer == nil {
		return nil, problem.ErrInternal(problem.Describe("console sessions are not configured"))
	}

	resolved, err := e.resolve(ctx, identity)
	if err != nil {
		return nil, err
	}

	result, err := e.openSession(ctx, r, identity, resolved)
	if err != nil {
		return nil, err
	}

	if w != nil {
		SetConsoleSessionCookie(w, r, result.Token, result.ExpiresAt)
	}
	return result, nil
}

// Upgrade re-proves a credential that is being MIGRATED rather than freshly
// presented, returning a console session for it without touching the response.
//
// It differs from [Exchanger.Exchange] in one way that matters: it reuses the
// session the browser already holds when there is one, extending its idle window
// instead of opening another. A login is an event and deserves its own session;
// a migration is the same session arriving under a different proof.
//
// An impersonated credential never reuses, so its clamped, non-refreshable
// lifetime is always computed from scratch rather than inherited from an
// ordinary session.
func (e *Exchanger) Upgrade(ctx context.Context, r *http.Request, identity *VerifiedIdentity) (*ExchangeResult, error) {
	if e.signer == nil {
		return nil, problem.ErrInternal(problem.Describe("console sessions are not configured"))
	}

	resolved, err := e.resolve(ctx, identity)
	if err != nil {
		return nil, err
	}

	if identity.Actor == nil {
		session, err := e.mgmt.ReuseAdminSession(ctx, resolved.adminID, resolved.identityID, time.Now().Add(e.signer.IdleTTL()))
		switch {
		case err == nil:
			// Reuse is not a login, so the identity's last_login_at is left
			// alone: nobody signed in here, an existing session simply arrived
			// under a new proof.
			return e.mint(session, identity)
		case !errors.Is(err, sql.ErrNoRows):
			return nil, err
		}
	}

	return e.openSession(ctx, r, identity, resolved)
}

// openSession records a new session for a resolved identity and mints its token.
func (e *Exchanger) openSession(ctx context.Context, r *http.Request, identity *VerifiedIdentity, resolved *resolvedIdentity) (*ExchangeResult, error) {
	if err := e.mgmt.TouchAdminIdentity(ctx, resolved.identityID, identity.Email, identity.EmailVerified); err != nil {
		return nil, err
	}

	session, err := e.recordSession(ctx, r, identity, resolved)
	if err != nil {
		return nil, err
	}

	return e.mint(session, identity)
}

// mint issues the token that proves a session. The token's expiry is the
// session's, so a credential can never outlive the row it names.
func (e *Exchanger) mint(session *management.AdminSession, identity *VerifiedIdentity) (*ExchangeResult, error) {
	token, err := e.signer.Mint(session, []string{identity.Provider})
	if err != nil {
		return nil, err
	}
	return &ExchangeResult{Token: token, ExpiresAt: session.ExpiresAt, Session: session}, nil
}

// Provision resolves a verified identity to an admin WITHOUT minting a session,
// creating the admin if none of the resolution steps found one.
//
// It exists for upstream webhooks that mirror a user before their first login.
// Sharing [Exchanger.resolve] with the login path is the point: a webhook must
// not be able to create a second admin for someone a login would have linked to
// an existing account.
func (e *Exchanger) Provision(ctx context.Context, identity *VerifiedIdentity) (uuid.UUID, error) {
	resolved, err := e.resolve(ctx, identity)
	if err != nil {
		return uuid.Nil, err
	}
	return resolved.adminID, nil
}

// resolvedIdentity is the admin the credential belongs to and the identity row
// that proves it.
type resolvedIdentity struct {
	adminID    uuid.UUID
	identityID uuid.UUID
}

// resolve maps a verified identity onto an admin, in strict order of certainty:
//
//  1. the exact (issuer, subject) identity -- the steady state,
//  2. a legacy identity carrying the sentinel issuer -- transitional,
//  3. an admin owning the same, PROVIDER-VERIFIED email address,
//  4. a brand-new admin and organization.
//
// Step 4 is unreachable for an impersonated session: impersonation assumes an
// identity, it never mints one.
func (e *Exchanger) resolve(ctx context.Context, identity *VerifiedIdentity) (*resolvedIdentity, error) {
	if identity.Issuer == "" || identity.Subject == "" {
		return nil, problem.ErrUnauthorized(problem.Describe("credential carries no identity"))
	}

	existing, err := e.mgmt.GetAdminIdentity(ctx, identity.Issuer, identity.Subject)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if existing != nil {
		return &resolvedIdentity{adminID: existing.AdminID, identityID: existing.ID}, nil
	}

	adopted, err := e.adoptLegacyIdentity(ctx, identity)
	if err != nil {
		return nil, err
	}
	if adopted != nil {
		return adopted, nil
	}

	linked, err := e.linkByEmail(ctx, identity)
	if err != nil {
		return nil, err
	}
	if linked != nil {
		return linked, nil
	}

	if identity.Actor != nil {
		// Impersonation assumes an existing identity. Creating one here would
		// let an upstream operator conjure an admin (and an organization) that
		// nobody at the customer ever authorised, attributed to a person who
		// does not exist.
		e.logger.Warn("refusing to provision an admin for an impersonated session",
			zap.String("issuer", identity.Issuer),
			zap.String("impersonator", identity.Actor.Subject))
		return nil, problem.ErrForbidden(problem.Describe("impersonation cannot create an account"))
	}

	return e.provision(ctx, identity, nil)
}

// adoptLegacyIdentity claims a backfilled external_id row for the upstream that
// just proved the same subject, rewriting it in place to the real issuer.
//
// This is the one resolution step that matches on subject ALONE, so it is worth
// stating why that is acceptable rather than leaving it as an oversight. It is
// only consulted for a credential the single configured upstream already
// verified; the subjects in question are high-entropy opaque provider ids, not
// anything a user chooses; and the whole branch is gated behind
// AUTH_LEGACY_IDENTITY_ADOPTION, which defaults on now and ships off once no
// sentinel rows remain.
func (e *Exchanger) adoptLegacyIdentity(ctx context.Context, identity *VerifiedIdentity) (*resolvedIdentity, error) {
	if !e.legacyAdoption {
		return nil, nil
	}

	legacy, err := e.mgmt.GetAdminIdentity(ctx, management.LegacyExternalIDIssuer, identity.Subject)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := e.mgmt.AdoptLegacyIdentity(ctx, legacy.ID, identity.Issuer, identity.Provider); err != nil {
		return nil, err
	}

	e.logger.Info("adopted legacy identity",
		zap.String("admin_id", legacy.AdminID.String()),
		zap.String("issuer", identity.Issuer))

	return &resolvedIdentity{adminID: legacy.AdminID, identityID: legacy.ID}, nil
}

// linkByEmail attaches a new identity to the admin already owning the address.
//
// It runs ONLY on an email the provider itself asserts as verified. Without that
// gate, an upstream that lets a user type any address into their profile becomes
// a way to take over any admin account by claiming its address.
func (e *Exchanger) linkByEmail(ctx context.Context, identity *VerifiedIdentity) (*resolvedIdentity, error) {
	if !identity.EmailVerified || identity.Email == "" {
		return nil, nil
	}

	admin, err := e.mgmt.ResolveAdminByEmail(ctx, identity.Email)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, nil
	case errors.Is(err, management.ErrAmbiguousEmail):
		// The address is contested by a quarantined duplicate, so nobody can
		// say which account it identifies. Both ids are logged because an
		// operator has to reconcile them by hand.
		var ambiguous *management.AmbiguousEmailError
		fields := []zap.Field{zap.String("issuer", identity.Issuer)}
		if errors.As(err, &ambiguous) {
			ids := make([]string, len(ambiguous.AdminIDs))
			for i, id := range ambiguous.AdminIDs {
				ids[i] = id.String()
			}
			fields = append(fields, zap.Strings("admin_ids", ids))
		}
		e.logger.Error("refusing to link an identity onto a contested email address", fields...)
		return nil, problem.ErrConflict(problem.Describe("this email address is registered more than once; contact your administrator"))
	case err != nil:
		return nil, err
	}

	identityID, err := e.mgmt.CreateAdminIdentity(ctx, management.AdminIdentity{
		AdminID:       admin.ID,
		Provider:      identity.Provider,
		Issuer:        identity.Issuer,
		Subject:       identity.Subject,
		Email:         &identity.Email,
		EmailVerified: identity.EmailVerified,
	})
	if err != nil {
		return nil, err
	}

	e.logger.Info("linked a new identity by verified email",
		zap.String("admin_id", admin.ID.String()),
		zap.String("issuer", identity.Issuer))

	return &resolvedIdentity{adminID: admin.ID, identityID: identityID}, nil
}

// LocalCredential supplies the parts of a brand-new identity that cannot be
// known before the admin exists: the subject it is keyed on, and the hash of the
// secret that proves it.
//
// A federated identity has neither -- its subject was minted upstream and its
// secret is not ours to hold -- and passes nil. A local one (email and password)
// is keyed on the admin's own id, which is why it has to be supplied here rather
// than on the [VerifiedIdentity]: the identity row and the admin it belongs to
// are created in the same statement pair, inside one transaction.
type LocalCredential func(adminID uuid.UUID) (subject string, secretHash string, err error)

// ProvisionAdmin creates an admin, the organization the [OrgResolver] picks for
// them and their first identity, then grants membership.
//
// It is the ONE path that creates an admin. A login reaches it through
// [Exchanger.Exchange] when no resolution step matched an existing account;
// local registration calls it directly, because a password identity is created
// together with its secret rather than proved before it exists. Sharing it is
// what keeps "who may be created, in which organization, with which role" a
// single answer instead of one per entry point.
//
// The organization, admin and identity all commit together or not at all, and
// the RBAC tuples are written strictly after that commit -- see
// [access.ProvisionMembership].
func (e *Exchanger) ProvisionAdmin(ctx context.Context, identity *VerifiedIdentity, credential LocalCredential) (adminID uuid.UUID, identityID uuid.UUID, err error) {
	resolved, err := e.provision(ctx, identity, credential)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return resolved.adminID, resolved.identityID, nil
}

func (e *Exchanger) provision(ctx context.Context, identity *VerifiedIdentity, credential LocalCredential) (*resolvedIdentity, error) {
	if identity.Email == "" {
		return nil, problem.ErrBadRequest(problem.Describe("the identity provider supplied no email address"))
	}
	if identity.Issuer == "" {
		return nil, problem.ErrUnauthorized(problem.Describe("credential carries no identity"))
	}
	if identity.Subject == "" && credential == nil {
		return nil, problem.ErrUnauthorized(problem.Describe("credential carries no identity"))
	}

	// The address is already somebody's. Resolution reached this far only
	// because it could not be linked -- either the upstream did not verify the
	// address, or the address is contested -- so provisioning would either fail
	// on the unique index or, worse, hand the claimant a second account on an
	// address that already identifies a person.
	if _, err := e.mgmt.ResolveAdminByEmail(ctx, identity.Email); !errors.Is(err, sql.ErrNoRows) {
		if err == nil || errors.Is(err, management.ErrAmbiguousEmail) {
			e.logger.Warn("refusing to provision an admin on an address that is already registered",
				zap.String("issuer", identity.Issuer))
			return nil, problem.ErrConflict(problem.Describe("this email address is already registered"))
		}
		return nil, err
	}

	var identityID uuid.UUID
	var membership access.Membership

	adminID, err := access.ProvisionMembership(ctx, e.db, e.engine,
		func(ctx context.Context, tx *management.State) (uuid.UUID, access.Membership, error) {
			organizationID, role, err := e.orgs.Resolve(ctx, tx, identity)
			if err != nil {
				return uuid.Nil, access.Membership{}, err
			}
			membership = access.Membership{OrganizationID: organizationID, Role: role}

			adminID, err := tx.CreateAdmin(ctx, management.Admin{
				OrganizationID: organizationID,
				Email:          identity.Email,
				FirstName:      identity.FirstName,
				LastName:       identity.LastName,
				ImageURL:       identity.ImageURL,
				Role:           role,
			})
			if err != nil {
				return uuid.Nil, access.Membership{}, err
			}

			row := management.AdminIdentity{
				AdminID:       adminID,
				Provider:      identity.Provider,
				Issuer:        identity.Issuer,
				Subject:       identity.Subject,
				Email:         &identity.Email,
				EmailVerified: identity.EmailVerified,
			}

			if credential != nil {
				subject, secretHash, err := credential(adminID)
				if err != nil {
					return uuid.Nil, access.Membership{}, err
				}
				row.Subject = subject
				row.SecretHash = &secretHash
			}

			identityID, err = tx.CreateAdminIdentity(ctx, row)
			if err != nil {
				return uuid.Nil, access.Membership{}, err
			}

			return adminID, membership, nil
		})
	if err != nil {
		return nil, err
	}

	e.logger.Info("provisioned a new admin",
		zap.String("admin_id", adminID.String()),
		zap.String("organization_id", membership.OrganizationID.String()),
		zap.String("issuer", identity.Issuer))

	return &resolvedIdentity{adminID: adminID, identityID: identityID}, nil
}

// recordSession writes the admin_sessions row the token will name.
//
// An impersonated session is clamped to the upstream session's expiry and marked
// non-refreshable, so it can neither outlive nor be extended past the session
// that authorised it. The same invariants are CHECK constraints on the table:
// this code computing them is the convenience, the constraints are the guarantee.
func (e *Exchanger) recordSession(ctx context.Context, r *http.Request, identity *VerifiedIdentity, resolved *resolvedIdentity) (*management.AdminSession, error) {
	now := time.Now()
	session := management.AdminSession{
		AdminID:           resolved.adminID,
		AdminIdentityID:   &resolved.identityID,
		ExpiresAt:         now.Add(e.signer.IdleTTL()),
		AbsoluteExpiresAt: now.Add(e.signer.AbsoluteTTL()),
		Refreshable:       true,
		UserAgent:         requestUserAgent(r),
		IP:                requestIP(r),
	}

	if identity.Actor != nil {
		if identity.ExpiresAt.IsZero() {
			return nil, problem.ErrForbidden(problem.Describe("an impersonated session must carry an expiry"))
		}

		upstream := identity.ExpiresAt
		session.Impersonated = true
		session.ImpersonatorSubject = &identity.Actor.Subject
		session.UpstreamExpiresAt = &upstream
		session.Refreshable = false
		session.ExpiresAt = earliest(session.ExpiresAt, upstream)
		session.AbsoluteExpiresAt = earliest(session.AbsoluteExpiresAt, upstream)

		if impersonator := e.resolveImpersonator(ctx, identity, resolved.adminID); impersonator != nil {
			session.ImpersonatorAdminID = impersonator
		}
	}

	return e.mgmt.CreateAdminSession(ctx, session)
}

// resolveImpersonator maps the upstream impersonator's subject onto one of our
// admins when it happens to be one. It usually is not -- the impersonator is
// typically a provider-dashboard user with no account here -- so a miss is
// ordinary and the raw subject recorded on the session remains the attribution.
func (e *Exchanger) resolveImpersonator(ctx context.Context, identity *VerifiedIdentity, impersonatedAdminID uuid.UUID) *uuid.UUID {
	row, err := e.mgmt.GetAdminIdentity(ctx, identity.Issuer, identity.Actor.Subject)
	if err != nil || row == nil {
		return nil
	}
	if row.AdminID == impersonatedAdminID {
		return nil
	}
	return &row.AdminID
}

func earliest(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

func requestUserAgent(r *http.Request) *string {
	if r == nil {
		return nil
	}
	agent := r.UserAgent()
	if agent == "" {
		return nil
	}
	if len(agent) > 512 {
		agent = agent[:512]
	}
	return &agent
}

// requestIP records the transport peer, not X-Forwarded-For. A forwarded header
// is attacker-controlled and this value is written to an INET column that feeds
// audit trails; a spoofable value in an audit trail is worse than none.
func requestIP(r *http.Request) *string {
	if r == nil {
		return nil
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if net.ParseIP(host) == nil {
		return nil
	}
	return &host
}
