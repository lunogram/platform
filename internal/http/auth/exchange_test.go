package auth

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const exchangeTestIssuer = "https://idp.test"

type exchangeEnv struct {
	db        *sqlx.DB
	mgmt      *management.State
	engine    *rbac.Engine
	exchanger *Exchanger
	signer    *ConsoleSigner
}

func newExchangeEnv(t *testing.T) *exchangeEnv {
	t.Helper()

	mgmtDB, _, _ := teststore.RunPostgreSQL(t)
	mgmt := management.NewState(mgmtDB)
	engine := rbac.NewTestEngine(t)
	signer := testConsoleSigner(t)

	return &exchangeEnv{
		db:        mgmtDB,
		mgmt:      mgmt,
		engine:    engine,
		signer:    signer,
		exchanger: NewExchanger(mgmtDB, mgmt, engine, signer, nil, zaptest.NewLogger(t), true),
	}
}

func (e *exchangeEnv) exchange(t *testing.T, identity *VerifiedIdentity) (*ExchangeResult, error) {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login/clerk/callback", nil)
	r.RemoteAddr = "203.0.113.9:44212"
	return e.exchanger.Exchange(context.Background(), httptest.NewRecorder(), r, identity)
}

func verifiedIdentity(subject, email string) *VerifiedIdentity {
	return &VerifiedIdentity{
		Issuer:        testIssuer,
		Subject:       subject,
		Provider:      management.IdentityProviderClerk,
		Email:         email,
		EmailVerified: true,
		ExpiresAt:     time.Now().Add(time.Hour),
	}
}

// TestExchangeResolutionOrder walks the four resolution steps in the order the
// exchange must apply them, from most certain to least.
func TestExchangeResolutionOrder(t *testing.T) {
	t.Parallel()
	env := newExchangeEnv(t)
	ctx := context.Background()

	t.Run("provisions a brand-new admin and organization", func(t *testing.T) {
		result, err := env.exchange(t, verifiedIdentity("user_new", "new@example.com"))
		require.NoError(t, err)
		require.NotNil(t, result.Session)

		admin, err := env.mgmt.GetAdmin(ctx, result.Session.AdminID)
		require.NoError(t, err)
		assert.Equal(t, "new@example.com", admin.Email)
		assert.Equal(t, rbac.OrganizationOwner, admin.Role)

		member, err := env.mgmt.IsMember(ctx, admin.OrganizationID, admin.ID)
		require.NoError(t, err)
		assert.True(t, member, "a provisioned admin must be a member of the organization they own")

		identity, err := env.mgmt.GetAdminIdentity(ctx, testIssuer, "user_new")
		require.NoError(t, err)
		assert.Equal(t, admin.ID, identity.AdminID)
	})

	t.Run("the exact identity resolves on every later login", func(t *testing.T) {
		first, err := env.exchange(t, verifiedIdentity("user_repeat", "repeat@example.com"))
		require.NoError(t, err)

		second, err := env.exchange(t, verifiedIdentity("user_repeat", "repeat@example.com"))
		require.NoError(t, err)

		assert.Equal(t, first.Session.AdminID, second.Session.AdminID,
			"a second login must reuse the admin, not provision another")
		assert.NotEqual(t, first.Session.ID, second.Session.ID, "each login is its own session")
	})

	t.Run("adopts a legacy identity in place", func(t *testing.T) {
		orgID, err := env.mgmt.CreateOrganization(ctx, "Legacy Org")
		require.NoError(t, err)
		adminID, err := env.mgmt.CreateAdmin(ctx, management.Admin{
			OrganizationID: orgID, Email: "legacy@example.com", Role: "owner",
		})
		require.NoError(t, err)
		identityID, err := env.mgmt.CreateAdminIdentity(ctx, management.AdminIdentity{
			AdminID:  adminID,
			Provider: management.IdentityProviderClerk,
			Issuer:   management.LegacyExternalIDIssuer,
			Subject:  "user_legacy",
		})
		require.NoError(t, err)

		result, err := env.exchange(t, verifiedIdentity("user_legacy", "legacy@example.com"))
		require.NoError(t, err)
		assert.Equal(t, adminID, result.Session.AdminID, "the backfilled admin must be adopted, not duplicated")

		adopted, err := env.mgmt.GetAdminIdentity(ctx, testIssuer, "user_legacy")
		require.NoError(t, err)
		assert.Equal(t, identityID, adopted.ID, "adoption rewrites the row in place so its id survives")
	})

	t.Run("links a new identity onto a verified email", func(t *testing.T) {
		orgID, err := env.mgmt.CreateOrganization(ctx, "Link Org")
		require.NoError(t, err)
		adminID, err := env.mgmt.CreateAdmin(ctx, management.Admin{
			OrganizationID: orgID, Email: "link@example.com", Role: "owner",
		})
		require.NoError(t, err)

		result, err := env.exchange(t, verifiedIdentity("user_link", "LINK@example.com"))
		require.NoError(t, err)
		assert.Equal(t, adminID, result.Session.AdminID,
			"the address must match case-insensitively, as the index now does")

		identity, err := env.mgmt.GetAdminIdentity(ctx, testIssuer, "user_link")
		require.NoError(t, err)
		assert.Equal(t, adminID, identity.AdminID)
	})
}

// TestExchangeDoesNotLinkUnverifiedEmail is the account-takeover guard: an
// upstream that lets a user type any address into their profile must not
// thereby hand them the account that already owns it.
func TestExchangeDoesNotLinkUnverifiedEmail(t *testing.T) {
	t.Parallel()
	env := newExchangeEnv(t)
	ctx := context.Background()

	orgID, err := env.mgmt.CreateOrganization(ctx, "Victim Org")
	require.NoError(t, err)
	victimID, err := env.mgmt.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID, Email: "victim@example.com", Role: "owner",
	})
	require.NoError(t, err)

	identity := verifiedIdentity("user_attacker", "victim@example.com")
	identity.EmailVerified = false

	// The claim is refused outright rather than linked or provisioned: linking
	// would hand over the victim's account, and provisioning would put a second
	// admin on an address that already identifies a person.
	_, err = env.exchange(t, identity)
	require.Error(t, err)
	assert.Equal(t, http.StatusConflict, problem.GetStatus(err))

	_, err = env.mgmt.GetAdminIdentity(ctx, exchangeTestIssuer, "user_attacker")
	require.ErrorIs(t, err, sql.ErrNoRows, "no identity may have been linked to the victim")

	victim, err := env.mgmt.GetAdmin(ctx, victimID)
	require.NoError(t, err)
	assert.Equal(t, "victim@example.com", victim.Email)

	// A VERIFIED claim on the same address does link, which is what makes the
	// unverified case a deliberate refusal rather than an accident.
	result, err := env.exchange(t, verifiedIdentity("user_owner", "victim@example.com"))
	require.NoError(t, err)
	assert.Equal(t, victimID, result.Session.AdminID)
}

// TestExchangeRefusesContestedEmail covers the quarantined-duplicate case: with
// two live admins sharing an address, nobody can say which account it names, so
// linking fails closed with a conflict rather than guessing.
func TestExchangeRefusesContestedEmail(t *testing.T) {
	t.Parallel()
	env := newExchangeEnv(t)
	ctx := context.Background()

	orgID, err := env.mgmt.CreateOrganization(ctx, "Contested Org")
	require.NoError(t, err)

	keeper, err := env.mgmt.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID, Email: "contested@example.com", Role: "owner",
	})
	require.NoError(t, err)

	// Reproduce what the reconciliation migration leaves behind: a second live
	// admin on the same address, quarantined. It has to be written already
	// quarantined, because the new index is exactly what stops a second
	// non-quarantined row from existing.
	_, err = env.db.ExecContext(ctx, `
		INSERT INTO admins (organization_id, active_organization_id, email, role, email_conflict_at)
		VALUES ($1, $1, 'Contested@Example.com', 'owner', NOW())`, orgID)
	require.NoError(t, err)

	_, err = env.exchange(t, verifiedIdentity("user_contested", "contested@example.com"))
	require.Error(t, err)
	assert.Equal(t, http.StatusConflict, problem.GetStatus(err))

	// Neither account was touched.
	admin, err := env.mgmt.GetAdmin(ctx, keeper)
	require.NoError(t, err)
	assert.Equal(t, "contested@example.com", admin.Email)

	_, err = env.mgmt.GetAdminIdentity(ctx, testIssuer, "user_contested")
	require.ErrorIs(t, err, sql.ErrNoRows)
}

// TestExchangeImpersonation covers the two rules that make impersonation safe:
// it may never create an account, and it may never outlive or outlast the
// upstream session that authorised it.
func TestExchangeImpersonation(t *testing.T) {
	t.Parallel()
	env := newExchangeEnv(t)
	ctx := context.Background()

	t.Run("cannot provision a new admin", func(t *testing.T) {
		identity := verifiedIdentity("user_unknown", "unknown@example.com")
		identity.Actor = &VerifiedActor{Subject: "user_support_engineer"}

		_, err := env.exchange(t, identity)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, problem.GetStatus(err),
			"impersonation assumes an identity, it never mints one")

		_, err = env.mgmt.ResolveAdminByEmail(ctx, "unknown@example.com")
		require.ErrorIs(t, err, sql.ErrNoRows, "no admin may have been created")
	})

	t.Run("clamps the lifetime to the upstream session and forbids refresh", func(t *testing.T) {
		// Establish the target admin with an ordinary login first.
		_, err := env.exchange(t, verifiedIdentity("user_target", "target@example.com"))
		require.NoError(t, err)

		upstreamExpiry := time.Now().Add(20 * time.Minute)
		identity := verifiedIdentity("user_target", "target@example.com")
		identity.Actor = &VerifiedActor{Subject: "user_support_engineer"}
		identity.ExpiresAt = upstreamExpiry

		result, err := env.exchange(t, identity)
		require.NoError(t, err)

		session := result.Session
		assert.True(t, session.Impersonated)
		assert.False(t, session.Refreshable, "an impersonated session must not be extendable")
		require.NotNil(t, session.ImpersonatorSubject)
		assert.Equal(t, "user_support_engineer", *session.ImpersonatorSubject)
		assert.WithinDuration(t, upstreamExpiry, session.ExpiresAt, time.Second)
		assert.WithinDuration(t, upstreamExpiry, session.AbsoluteExpiresAt, time.Second,
			"the session must not outlive the upstream session that authorised it")

		_, err = env.mgmt.RefreshAdminSession(ctx, session.ID, time.Now().Add(8*time.Hour))
		require.Error(t, err)
	})

	t.Run("records the impersonator when they map to an admin of ours", func(t *testing.T) {
		staff, err := env.exchange(t, verifiedIdentity("user_staff", "staff@example.com"))
		require.NoError(t, err)
		_, err = env.exchange(t, verifiedIdentity("user_customer", "customer@example.com"))
		require.NoError(t, err)

		identity := verifiedIdentity("user_customer", "customer@example.com")
		identity.Actor = &VerifiedActor{Subject: "user_staff"}
		identity.ExpiresAt = time.Now().Add(20 * time.Minute)

		result, err := env.exchange(t, identity)
		require.NoError(t, err)
		require.NotNil(t, result.Session.ImpersonatorAdminID)
		assert.Equal(t, staff.Session.AdminID, *result.Session.ImpersonatorAdminID)
	})

	t.Run("an impersonated session without an upstream expiry is refused", func(t *testing.T) {
		_, err := env.exchange(t, verifiedIdentity("user_noexp", "noexp@example.com"))
		require.NoError(t, err)

		identity := verifiedIdentity("user_noexp", "noexp@example.com")
		identity.Actor = &VerifiedActor{Subject: "user_support_engineer"}
		identity.ExpiresAt = time.Time{}

		_, err = env.exchange(t, identity)
		require.Error(t, err)
	})
}

// TestExchangeSessionLifetime covers the ordinary (non-impersonated) case.
func TestExchangeSessionLifetime(t *testing.T) {
	t.Parallel()
	env := newExchangeEnv(t)

	before := time.Now()
	result, err := env.exchange(t, verifiedIdentity("user_lifetime", "lifetime@example.com"))
	require.NoError(t, err)

	session := result.Session
	assert.True(t, session.Refreshable)
	assert.Nil(t, session.UpstreamExpiresAt)
	assert.WithinDuration(t, before.Add(env.signer.IdleTTL()), session.ExpiresAt, time.Minute)
	assert.WithinDuration(t, before.Add(env.signer.AbsoluteTTL()), session.AbsoluteExpiresAt, time.Minute)
	require.NotNil(t, session.IP)
	assert.Equal(t, "203.0.113.9", *session.IP)

	claims, err := env.signer.Verify(result.Token)
	require.NoError(t, err)
	assert.Equal(t, session.ID, claims.SessionID)
	assert.Equal(t, session.AdminID, claims.AdminID)
}

// TestExchangeLegacyAdoptionCanBeDisabled pins the kill switch: the one
// resolution step that matches on subject alone must be removable without a
// deploy of new code.
func TestExchangeLegacyAdoptionCanBeDisabled(t *testing.T) {
	t.Parallel()
	env := newExchangeEnv(t)
	ctx := context.Background()

	env.exchanger = NewExchanger(env.db, env.mgmt, env.engine, env.signer, nil, zaptest.NewLogger(t), false)

	orgID, err := env.mgmt.CreateOrganization(ctx, "Disabled Org")
	require.NoError(t, err)
	adminID, err := env.mgmt.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID, Email: "disabled@example.com", Role: "owner",
	})
	require.NoError(t, err)
	_, err = env.mgmt.CreateAdminIdentity(ctx, management.AdminIdentity{
		AdminID:  adminID,
		Provider: management.IdentityProviderClerk,
		Issuer:   management.LegacyExternalIDIssuer,
		Subject:  "user_not_adopted",
		Email:    ptr.To("disabled@example.com"),
	})
	require.NoError(t, err)

	// The verified email still reaches the same admin: disabling adoption
	// removes the subject-only match, not the account.
	result, err := env.exchange(t, verifiedIdentity("user_not_adopted", "disabled@example.com"))
	require.NoError(t, err)
	assert.Equal(t, adminID, result.Session.AdminID)

	_, err = env.mgmt.GetAdminIdentity(ctx, testIssuer, "user_not_adopted")
	require.NoError(t, err, "the identity was linked by email, not adopted")

	legacy, err := env.mgmt.GetAdminIdentity(ctx, management.LegacyExternalIDIssuer, "user_not_adopted")
	require.NoError(t, err)
	assert.Equal(t, management.LegacyExternalIDIssuer, legacy.Issuer, "the sentinel row must be left alone")
}

func TestExchangeRejectsIdentityWithoutSubject(t *testing.T) {
	t.Parallel()
	env := newExchangeEnv(t)

	_, err := env.exchange(t, &VerifiedIdentity{Issuer: testIssuer, Email: "nobody@example.com"})
	require.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, problem.GetStatus(err))
}

func TestExchangeRequiresEmailToProvision(t *testing.T) {
	t.Parallel()
	env := newExchangeEnv(t)

	identity := verifiedIdentity("user_noemail", "")
	identity.EmailVerified = false

	_, err := env.exchange(t, identity)
	require.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, problem.GetStatus(err))
}

func TestExchangeSetsConsoleCookie(t *testing.T) {
	t.Parallel()
	env := newExchangeEnv(t)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/auth/login/clerk/callback", nil)
	r.Header.Set("X-Forwarded-Proto", "https")

	result, err := env.exchanger.Exchange(context.Background(), w, r, verifiedIdentity("user_cookie", "cookie@example.com"))
	require.NoError(t, err)

	cookies := w.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.Equal(t, ConsoleCookieSecure, cookies[0].Name,
		"the console cookie must not reuse __session, which the upstream SDK rewrites")
	assert.Equal(t, result.Token, cookies[0].Value)
	assert.True(t, cookies[0].HttpOnly)
}

func TestProvisionSharesTheLoginResolutionPath(t *testing.T) {
	t.Parallel()
	env := newExchangeEnv(t)
	ctx := context.Background()

	orgID, err := env.mgmt.CreateOrganization(ctx, "Webhook Org")
	require.NoError(t, err)
	adminID, err := env.mgmt.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID, Email: "webhook@example.com", Role: "owner",
	})
	require.NoError(t, err)

	// A webhook mirroring a user whose address already belongs to an admin must
	// link, exactly as a login would -- never create a second account.
	provisioned, err := env.exchanger.Provision(ctx, verifiedIdentity("user_webhook", "webhook@example.com"))
	require.NoError(t, err)
	assert.Equal(t, adminID, provisioned)

	var sessions int
	require.NoError(t, env.db.GetContext(ctx, &sessions,
		`SELECT count(*) FROM admin_sessions WHERE admin_id = $1`, adminID))
	assert.Zero(t, sessions, "mirroring a user must not open a session for them")
}

func TestExchangeRecordsLoginOnTheIdentity(t *testing.T) {
	t.Parallel()
	env := newExchangeEnv(t)
	ctx := context.Background()

	_, err := env.exchange(t, verifiedIdentity("user_touch", "touch@example.com"))
	require.NoError(t, err)

	identity, err := env.mgmt.GetAdminIdentity(ctx, testIssuer, "user_touch")
	require.NoError(t, err)
	require.NotNil(t, identity.LastLoginAt)
	assert.WithinDuration(t, time.Now(), *identity.LastLoginAt, time.Minute)
}

// A local credential cannot be proved before it exists, so registration creates
// the admin and the identity together and supplies the parts that depend on the
// admin id from inside the transaction.
func TestProvisionAdminWithALocalCredential(t *testing.T) {
	t.Parallel()
	env := newExchangeEnv(t)
	ctx := context.Background()

	adminID, identityID, err := env.exchanger.ProvisionAdmin(ctx, &VerifiedIdentity{
		Issuer:   management.LocalIssuer,
		Provider: management.IdentityProviderBasic,
		Email:    "local@example.test",
	}, Provisioning{Credential: func(adminID uuid.UUID) (string, string, error) {
		return adminID.String(), "$argon2id$v=19$m=65536,t=2,p=1$c2FsdHNhbHRzYWx0c2E$aGFzaGhhc2hoYXNoaGFzaA", nil
	}})
	require.NoError(t, err)

	identity, err := env.mgmt.GetLocalIdentity(ctx, adminID)
	require.NoError(t, err)
	assert.Equal(t, identityID, identity.ID)
	assert.Equal(t, adminID.String(), identity.Subject, "a local identity is keyed on the admin it belongs to")
	require.NotNil(t, identity.SecretHash)

	// A brand-new local account has proved nothing about its address.
	assert.False(t, identity.EmailVerified)

	admin, err := env.mgmt.GetAdmin(ctx, adminID)
	require.NoError(t, err)
	assert.Equal(t, "local@example.test", admin.Email)

	member, err := env.mgmt.IsMember(ctx, admin.OrganizationID, adminID)
	require.NoError(t, err)
	assert.True(t, member, "membership is granted in the same operation")
}

// The credential callback runs inside the provisioning transaction, so its
// failure has to take the admin and the organization down with it.
func TestProvisionAdminRollsBackWhenTheCredentialFails(t *testing.T) {
	t.Parallel()
	env := newExchangeEnv(t)
	ctx := context.Background()

	_, _, err := env.exchanger.ProvisionAdmin(ctx, &VerifiedIdentity{
		Issuer:   management.LocalIssuer,
		Provider: management.IdentityProviderBasic,
		Email:    "rollback@example.test",
	}, Provisioning{Credential: func(uuid.UUID) (string, string, error) {
		return "", "", errors.New("could not hash")
	}})
	require.Error(t, err)

	_, err = env.mgmt.ResolveAdminByEmail(ctx, "rollback@example.test")
	assert.ErrorIs(t, err, sql.ErrNoRows, "no admin may survive a failed credential")
}

// A federated identity has no local credential and must still be rejected when
// its upstream named no subject.
func TestProvisionAdminRequiresASubjectWithoutACredential(t *testing.T) {
	t.Parallel()
	env := newExchangeEnv(t)

	_, _, err := env.exchanger.ProvisionAdmin(context.Background(), &VerifiedIdentity{
		Issuer:   exchangeTestIssuer,
		Provider: management.IdentityProviderClerk,
		Email:    "nosubject@example.test",
	}, Provisioning{})
	require.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, problem.GetStatus(err))
}

// Somebody who was invited before they had an account must land in the
// organization that invited them, whichever driver they eventually sign up with.
func TestInviteOrgResolver(t *testing.T) {
	t.Parallel()
	env := newExchangeEnv(t)
	ctx := context.Background()

	orgID, err := env.mgmt.CreateOrganization(ctx, "Inviting Organization")
	require.NoError(t, err)
	inviterID, err := env.mgmt.CreateAdmin(ctx, management.Admin{
		OrganizationID: orgID, Email: "inviter@example.test", Role: rbac.OrganizationOwner,
	})
	require.NoError(t, err)
	projectID, err := env.mgmt.CreateProject(ctx, management.Project{
		Name: "Invited Project", Timezone: "UTC", Locale: "en", OrganizationID: &orgID,
	})
	require.NoError(t, err)
	_, err = env.mgmt.CreateProjectInvite(ctx, projectID, inviterID, "invited@example.test", nil, "editor", time.Hour)
	require.NoError(t, err)

	t.Run("a verified invited address joins the inviting organization as a member", func(t *testing.T) {
		organizationID, role, err := InviteOrgResolver{}.Resolve(ctx, env.mgmt,
			&VerifiedIdentity{Email: "invited@example.test", EmailVerified: true})
		require.NoError(t, err)
		assert.Equal(t, orgID, organizationID)
		assert.Equal(t, rbac.OrganizationMember, role)
	})

	// Confirming an address is no longer part of any flow, so an invited
	// registrant lands in the inviting organization either way. Requiring proof
	// here once meant every invitee also got a stray organization of their own,
	// and it protected nothing that accepting the invite does not already grant.
	t.Run("an unconfirmed invited address still joins the inviting organization", func(t *testing.T) {
		organizationID, role, err := InviteOrgResolver{}.Resolve(ctx, env.mgmt,
			&VerifiedIdentity{Email: "invited@example.test", EmailVerified: false})
		require.NoError(t, err)
		assert.Equal(t, orgID, organizationID)
		assert.Equal(t, rbac.OrganizationMember, role)
	})

	t.Run("anybody else gets an organization of their own and owns it", func(t *testing.T) {
		organizationID, role, err := InviteOrgResolver{}.Resolve(ctx, env.mgmt,
			&VerifiedIdentity{Email: "stranger@example.test", EmailVerified: true})
		require.NoError(t, err)
		assert.NotEqual(t, orgID, organizationID)
		assert.Equal(t, rbac.OrganizationOwner, role)
	})

	// The other half: once the address is proved, the invite is honoured.
	t.Run("AdmitInvitee grants membership once the address is proved", func(t *testing.T) {
		adminID, err := env.mgmt.CreateAdmin(ctx, management.Admin{
			OrganizationID: orgID, Email: "admitted@example.test", Role: rbac.OrganizationOwner,
		})
		require.NoError(t, err)
		_, err = env.mgmt.CreateProjectInvite(ctx, projectID, inviterID, "admitted@example.test", nil, "editor", time.Hour)
		require.NoError(t, err)

		require.NoError(t, env.exchanger.AdmitInvitee(ctx, adminID, "admitted@example.test"))

		member, err := env.mgmt.IsMember(ctx, orgID, adminID)
		require.NoError(t, err)
		assert.True(t, member)
	})

	// Re-running it must never rewrite a role. An owner who happens to hold a
	// pending invite to their own organization stays an owner.
	t.Run("AdmitInvitee never demotes an existing member", func(t *testing.T) {
		adminID, err := env.mgmt.CreateAdmin(ctx, management.Admin{
			OrganizationID: orgID, Email: "owner@example.test", Role: rbac.OrganizationOwner,
		})
		require.NoError(t, err)
		require.NoError(t, env.mgmt.AddMember(ctx, orgID, adminID, rbac.OrganizationOwner))
		_, err = env.mgmt.CreateProjectInvite(ctx, projectID, inviterID, "owner@example.test", nil, "editor", time.Hour)
		require.NoError(t, err)

		err = env.exchanger.AdmitInvitee(ctx, adminID, "owner@example.test")
		assert.ErrorIs(t, err, ErrAlreadyMember)

		member, err := env.mgmt.GetMember(ctx, orgID, adminID)
		require.NoError(t, err)
		assert.Equal(t, rbac.OrganizationOwner, member.Role, "an invite must not demote an owner")
	})

	t.Run("AdmitInvitee reports no pending invite", func(t *testing.T) {
		adminID, err := env.mgmt.CreateAdmin(ctx, management.Admin{
			OrganizationID: orgID, Email: "uninvited@example.test", Role: rbac.OrganizationOwner,
		})
		require.NoError(t, err)

		err = env.exchanger.AdmitInvitee(ctx, adminID, "uninvited@example.test")
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})
}
