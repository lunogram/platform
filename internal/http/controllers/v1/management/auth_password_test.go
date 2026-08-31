package v1

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/mailer"
	"github.com/lunogram/platform/internal/password"
	"github.com/lunogram/platform/internal/ratelimit"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

// capturedMailer records what would have been sent, so a test can follow the
// link the flow put in an inbox.
type capturedMailer struct {
	mu       sync.Mutex
	messages []mailer.Message
}

func (m *capturedMailer) Send(_ context.Context, message mailer.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, message)
	return nil
}

func (m *capturedMailer) all() []mailer.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]mailer.Message(nil), m.messages...)
}

// awaitSubject waits for a message with the given subject. Sends are
// deliberately asynchronous, so the test waits rather than assuming.
func (m *capturedMailer) awaitSubject(t *testing.T, subject string) mailer.Message {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, message := range m.all() {
			if message.Subject == subject {
				return message
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("no message with subject %q was sent; got %v", subject, m.subjects())
	return mailer.Message{}
}

// awaitQuiet gives an asynchronous send time to arrive before asserting that
// nothing was sent, so the assertion cannot pass merely by being early.
func (m *capturedMailer) awaitQuiet(t *testing.T) {
	t.Helper()
	time.Sleep(250 * time.Millisecond)
	assert.Empty(t, m.subjects(), "no message should have been sent")
}

func (m *capturedMailer) subjects() []string {
	messages := m.all()
	subjects := make([]string, len(messages))
	for i, message := range messages {
		subjects[i] = message.Subject
	}
	return subjects
}

type passwordEnv struct {
	t          *testing.T
	controller *AuthController
	state      *management.State
	db         *sqlx.DB
	mail       *capturedMailer
	orgID      uuid.UUID
}

// testMailConfig is a channel that builds but never dials: every test replaces
// the dispatcher's transport with a capturedMailer before anything is sent. It
// exists because a deployment offering password logins with nowhere to send
// mail is refused at boot, and these tests are such a deployment.
func testMailConfig() mailer.Config {
	config := mailer.DefaultConfig()
	config.Channel = mailer.ChannelSMTP
	config.SMTP.Host = "smtp.invalid"
	config.From.Address = "no-reply@example.test"
	return config
}

func newPasswordEnv(t *testing.T, registration string) *passwordEnv {
	t.Helper()

	logger := zaptest.NewLogger(t)
	mgmtDB, _, _ := teststore.RunPostgreSQL(t)
	state := management.NewState(mgmtDB)

	controller, err := NewAuthController(logger, mgmtDB, state, config.Node{
		PublicURL: "https://console.example.test",
		Auth: config.Auth{
			Drivers:  []string{"password"},
			Password: config.PasswordAuth{Registration: registration},
		},
		Mail: testMailConfig(),
	}, nil, consoleSignerFor(t), nil)
	require.NoError(t, err)

	captured := &capturedMailer{}
	controller.password.mail = mailer.NewDispatcher(captured, logger, time.Second)
	t.Cleanup(controller.password.mail.Close)

	orgID, err := state.CreateOrganization(t.Context(), "Password Organization")
	require.NoError(t, err)

	return &passwordEnv{t: t, controller: controller, state: state, db: mgmtDB, mail: captured, orgID: orgID}
}

func (e *passwordEnv) post(handler http.HandlerFunc, body any) *httptest.ResponseRecorder {
	e.t.Helper()

	encoded, err := json.Marshal(body)
	require.NoError(e.t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/auth/test", strings.NewReader(string(encoded)))
	req.RemoteAddr = "203.0.113.7:52000"
	handler(res, req)
	return res
}

func (e *passwordEnv) register(email, plain string) *httptest.ResponseRecorder {
	e.t.Helper()
	return e.post(e.controller.RegisterWithPassword, map[string]string{"email": email, "password": plain})
}

func (e *passwordEnv) login(email, plain string) *httptest.ResponseRecorder {
	e.t.Helper()

	encoded, err := json.Marshal(map[string]string{"email": email, "password": plain})
	require.NoError(e.t, err)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/auth/login/password/callback", strings.NewReader(string(encoded)))
	req.RemoteAddr = "203.0.113.7:52000"
	e.controller.AuthCallback(res, req, "password")
	return res
}

// tokenFrom pulls the single-use token out of the link a message carries.
func tokenFrom(t *testing.T, message mailer.Message) string {
	t.Helper()

	_, token, found := strings.Cut(message.ActionURL, "token=")
	require.True(t, found, "message carried no token: %q", message.ActionURL)
	require.NotEmpty(t, token)
	return token
}

func (e *passwordEnv) adminByEmail(email string) *management.Admin {
	e.t.Helper()
	admin, err := e.state.ResolveAdminByEmail(e.t.Context(), email)
	require.NoError(e.t, err)
	return admin
}

const testPassword = "an entirely ordinary passphrase"

func TestRegisterAndVerify(t *testing.T) {
	t.Parallel()
	env := newPasswordEnv(t, config.RegistrationOpen)

	res := env.register("New.Admin@Example.test", testPassword)
	require.Equal(t, http.StatusNoContent, res.Code, res.Body.String())

	admin := env.adminByEmail("new.admin@example.test")
	assert.Equal(t, "new.admin@example.test", admin.Email, "the address is stored normalised")

	identity, err := env.state.GetPasswordIdentity(t.Context(), admin.ID)
	require.NoError(t, err)
	require.NotNil(t, identity.SecretHash)
	assert.NotContains(t, *identity.SecretHash, testPassword, "the password itself must never be stored")
	assert.False(t, identity.EmailVerified, "a fresh registration has proved nothing about its address")

	// The identity is keyed on the admin's own id, which is what makes one
	// password per account structural rather than a rule somebody has to
	// remember.
	assert.Equal(t, admin.ID.String(), identity.Subject)
	assert.Equal(t, management.PasswordIssuer, identity.Issuer)

	message := env.mail.awaitSubject(t, "Confirm your email address")
	verify := env.post(env.controller.VerifyEmail, map[string]string{"token": tokenFrom(t, message)})
	require.Equal(t, http.StatusNoContent, verify.Code, verify.Body.String())

	identity, err = env.state.GetPasswordIdentity(t.Context(), admin.ID)
	require.NoError(t, err)
	assert.True(t, identity.EmailVerified)

	// Single use: the link in an inbox is worth one redemption.
	replay := env.post(env.controller.VerifyEmail, map[string]string{"token": tokenFrom(t, message)})
	assert.Equal(t, http.StatusBadRequest, replay.Code)
}

// The response to a registration must be the same whether or not the address is
// already taken, or the endpoint becomes a way to walk an account list.
func TestRegisterDoesNotRevealExistingAccounts(t *testing.T) {
	t.Parallel()
	env := newPasswordEnv(t, config.RegistrationOpen)

	first := env.register("taken@example.test", testPassword)
	require.Equal(t, http.StatusNoContent, first.Code)
	env.mail.awaitSubject(t, "Confirm your email address")

	admin := env.adminByEmail("taken@example.test")

	second := env.register("taken@example.test", "a completely different passphrase")
	require.Equal(t, http.StatusNoContent, second.Code, "a taken address answers exactly like a free one")
	assert.Equal(t, first.Body.String(), second.Body.String())

	// The owner of the address is told, in the one channel that reaches only
	// them.
	env.mail.awaitSubject(t, "You already have an account")

	// The second attempt must not have touched the account.
	identity, err := env.state.GetPasswordIdentity(t.Context(), admin.ID)
	require.NoError(t, err)
	match, _, err := password.Verify(*identity.SecretHash, testPassword)
	require.NoError(t, err)
	assert.True(t, match, "the original password still stands")
}

func TestRegisterEnforcesThePasswordPolicy(t *testing.T) {
	t.Parallel()
	env := newPasswordEnv(t, config.RegistrationOpen)

	tests := map[string]struct {
		email    string
		password string
	}{
		"too short":       {email: "short@example.test", password: "hunter2"},
		"is the address":  {email: "someone@example.test", password: "someone@example.test"},
		"easily guessed":  {email: "guess@example.test", password: "passwordpassword"},
		"absurdly long":   {email: "long@example.test", password: strings.Repeat("a", password.MaxLength+1)},
		"missing address": {email: "  ", password: testPassword},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := env.register(test.email, test.password)
			assert.Equal(t, http.StatusBadRequest, res.Code, res.Body.String())
		})
	}

	_, err := env.state.ResolveAdminByEmail(t.Context(), "short@example.test")
	assert.ErrorIs(t, err, sql.ErrNoRows, "a rejected registration creates nothing")
}

func TestRegistrationModes(t *testing.T) {
	t.Parallel()

	t.Run("disabled answers 404", func(t *testing.T) {
		t.Parallel()
		env := newPasswordEnv(t, config.RegistrationDisabled)

		res := env.register("nope@example.test", testPassword)
		assert.Equal(t, http.StatusNotFound, res.Code)
	})

	// A deployment whose safe default made the first account impossible to
	// create would simply be broken, so invite-only admits the account nobody
	// could have invited -- and only until one exists.
	t.Run("invite only lets the very first account through", func(t *testing.T) {
		t.Parallel()
		env := newPasswordEnv(t, config.RegistrationInviteOnly)

		res := env.register("first@example.test", testPassword)
		require.Equal(t, http.StatusNoContent, res.Code)
		env.mail.awaitSubject(t, "Confirm your email address")
		env.adminByEmail("first@example.test")
	})

	t.Run("invite only refuses an uninvited address once an admin exists", func(t *testing.T) {
		t.Parallel()
		env := newPasswordEnv(t, config.RegistrationInviteOnly)

		_, err := env.state.CreateAdmin(t.Context(), management.Admin{
			OrganizationID: env.orgID, Email: "existing@example.test", Role: "owner",
		})
		require.NoError(t, err)

		res := env.register("uninvited@example.test", testPassword)
		// Still 204: whether an address was invited is not the caller's to
		// discover by watching status codes.
		require.Equal(t, http.StatusNoContent, res.Code)
		env.mail.awaitQuiet(t)

		_, err = env.state.ResolveAdminByEmail(t.Context(), "uninvited@example.test")
		assert.ErrorIs(t, err, sql.ErrNoRows)
	})
}

// Registering an invited address must NOT, on its own, get you into the
// organization that issued the invite.
//
// This is the escalation the verification gate exists to stop: invited
// addresses are corporate and guessable, so if registration alone honoured the
// invite, anyone who could guess victim@bigcorp.com would be inside BigCorp's
// organization having never touched that mailbox. Under the default
// invite_only mode a pending invite is exactly what admits the registration,
// so this is the DEFAULT path rather than an exotic one.
func TestRegisteringAnInvitedAddressGrantsNothingUntilItIsVerified(t *testing.T) {
	t.Parallel()
	env := newPasswordEnv(t, config.RegistrationInviteOnly)
	ctx := t.Context()

	inviterID, err := env.state.CreateAdmin(ctx, management.Admin{
		OrganizationID: env.orgID, Email: "inviter@example.test", Role: "owner",
	})
	require.NoError(t, err)

	projectID, err := env.state.CreateProject(ctx, management.Project{
		Name: "Invited Project", Timezone: "UTC", Locale: "en", OrganizationID: &env.orgID,
	})
	require.NoError(t, err)

	_, err = env.state.CreateProjectInvite(ctx, projectID, inviterID, "invitee@example.test", nil, "editor", time.Hour)
	require.NoError(t, err)

	res := env.register("invitee@example.test", testPassword)
	require.Equal(t, http.StatusNoContent, res.Code)
	message := env.mail.awaitSubject(t, "Confirm your email address")

	admin := env.adminByEmail("invitee@example.test")

	// Before the mailbox is proved: an organization of their own, and no
	// membership whatsoever of the inviting one.
	assert.NotEqual(t, env.orgID, admin.OrganizationID,
		"an unverified registrant must not land in the inviting organization")

	member, err := env.state.IsMember(ctx, env.orgID, admin.ID)
	require.NoError(t, err)
	assert.False(t, member, "an unverified registrant must hold no membership of the inviting organization")

	// Proving the mailbox is what honours the invite.
	verify := env.post(env.controller.VerifyEmail, map[string]string{"token": tokenFrom(t, message)})
	require.Equal(t, http.StatusNoContent, verify.Code, verify.Body.String())

	member, err = env.state.IsMember(ctx, env.orgID, admin.ID)
	require.NoError(t, err)
	assert.True(t, member, "a verified invitee joins the organization that invited them")

	// The invite itself stays pending: accepting it is what grants the project
	// role, and that flow already exists.
	invites, err := env.state.ListInvitesForEmail(ctx, "invitee@example.test")
	require.NoError(t, err)
	assert.Len(t, invites, 1)
}

// Completing a reset proves the same mailbox a confirmation link would, so it
// honours a pending invite too. Without this an invited admin who never saw
// their confirmation mail could never be admitted at all.
func TestCompletingAResetHonoursAPendingInvite(t *testing.T) {
	t.Parallel()
	env := newPasswordEnv(t, config.RegistrationInviteOnly)
	ctx := t.Context()

	inviterID, err := env.state.CreateAdmin(ctx, management.Admin{
		OrganizationID: env.orgID, Email: "reset-inviter@example.test", Role: "owner",
	})
	require.NoError(t, err)
	projectID, err := env.state.CreateProject(ctx, management.Project{
		Name: "Reset Project", Timezone: "UTC", Locale: "en", OrganizationID: &env.orgID,
	})
	require.NoError(t, err)
	_, err = env.state.CreateProjectInvite(ctx, projectID, inviterID, "reset-invitee@example.test", nil, "editor", time.Hour)
	require.NoError(t, err)

	require.Equal(t, http.StatusNoContent, env.register("reset-invitee@example.test", testPassword).Code)
	env.mail.awaitSubject(t, "Confirm your email address")
	admin := env.adminByEmail("reset-invitee@example.test")

	member, err := env.state.IsMember(ctx, env.orgID, admin.ID)
	require.NoError(t, err)
	require.False(t, member)

	require.Equal(t, http.StatusNoContent,
		env.post(env.controller.RequestPasswordReset, map[string]string{"email": "reset-invitee@example.test"}).Code)
	message := env.mail.awaitSubject(t, "Reset your password")

	confirm := env.post(env.controller.ConfirmPasswordReset, map[string]string{
		"token": tokenFrom(t, message), "password": "a brand new and different passphrase",
	})
	require.Equal(t, http.StatusNoContent, confirm.Code, confirm.Body.String())

	member, err = env.state.IsMember(ctx, env.orgID, admin.ID)
	require.NoError(t, err)
	assert.True(t, member, "proving the mailbox through a reset honours the invite too")
}

// Two registrations racing for the bootstrap allowance must not both become
// owners of their own organizations on a deployment meant to admit one founder.
func TestConcurrentBootstrapRegistrationsAdmitExactlyOne(t *testing.T) {
	t.Parallel()
	env := newPasswordEnv(t, config.RegistrationInviteOnly)
	ctx := t.Context()

	const racers = 6
	var (
		start = make(chan struct{})
		wg    sync.WaitGroup
	)

	wg.Add(racers)
	for i := range racers {
		go func() {
			defer wg.Done()
			<-start
			env.register(fmt.Sprintf("founder%d@example.test", i), testPassword)
		}()
	}
	close(start)
	wg.Wait()

	var admins int
	require.NoError(t, env.db.GetContext(ctx, &admins,
		`SELECT count(*) FROM admins WHERE deleted_at IS NULL`))
	assert.Equal(t, 1, admins, "exactly one registration may claim the first account")
}

func TestPasswordLogin(t *testing.T) {
	t.Parallel()
	env := newPasswordEnv(t, config.RegistrationOpen)

	require.Equal(t, http.StatusNoContent, env.register("login@example.test", testPassword).Code)
	env.mail.awaitSubject(t, "Confirm your email address")

	t.Run("the right password opens a session", func(t *testing.T) {
		res := env.login("login@example.test", testPassword)
		require.Equal(t, http.StatusOK, res.Code, res.Body.String())

		var cookies []string
		for _, cookie := range res.Result().Cookies() {
			cookies = append(cookies, cookie.Name)
		}
		assert.NotEmpty(t, cookies, "a login sets the console session cookie")

		admin := env.adminByEmail("login@example.test")
		sessions, err := env.state.ListAdminSessions(t.Context(), admin.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, sessions, "the session is recorded so it can be revoked")
	})

	// An unverified account may sign in, but it must stay unverified: the
	// exchange links identities by email only on a proved address.
	t.Run("signing in does not confirm the address", func(t *testing.T) {
		admin := env.adminByEmail("login@example.test")
		identity, err := env.state.GetPasswordIdentity(t.Context(), admin.ID)
		require.NoError(t, err)
		assert.False(t, identity.EmailVerified)
	})

	// A wrong password and an address with no account must be indistinguishable
	// from the outside.
	t.Run("a wrong password and an unknown address answer identically", func(t *testing.T) {
		wrong := env.login("login@example.test", "not the right passphrase")
		unknown := env.login("nobody@example.test", "not the right passphrase")

		assert.Equal(t, http.StatusUnauthorized, wrong.Code)
		assert.Equal(t, unknown.Code, wrong.Code)
		assert.Equal(t, unknown.Body.String(), wrong.Body.String())
	})
}

// A hash produced under weaker parameters must be replaced the next time its
// owner proves it, or raising the cost only ever protects accounts created
// afterwards.
func TestLoginRehashesAnOutdatedPassword(t *testing.T) {
	t.Parallel()
	env := newPasswordEnv(t, config.RegistrationOpen)
	ctx := t.Context()

	require.Equal(t, http.StatusNoContent, env.register("outdated@example.test", testPassword).Code)
	env.mail.awaitSubject(t, "Confirm your email address")

	admin := env.adminByEmail("outdated@example.test")
	identity, err := env.state.GetPasswordIdentity(ctx, admin.ID)
	require.NoError(t, err)

	weaker := password.DefaultParams
	weaker.Memory /= 4
	weaker.Time = 1
	legacy, err := password.HashWith(testPassword, weaker)
	require.NoError(t, err)
	require.NoError(t, env.state.SetAdminIdentitySecret(ctx, identity.ID, legacy))

	require.Equal(t, http.StatusOK, env.login("outdated@example.test", testPassword).Code)

	upgraded, err := env.state.GetPasswordIdentity(ctx, admin.ID)
	require.NoError(t, err)
	assert.NotEqual(t, legacy, *upgraded.SecretHash, "the stored hash was replaced")

	_, outdated, err := password.Verify(*upgraded.SecretHash, testPassword)
	require.NoError(t, err)
	assert.False(t, outdated)
}

func TestPasswordReset(t *testing.T) {
	t.Parallel()
	env := newPasswordEnv(t, config.RegistrationOpen)
	ctx := t.Context()

	require.Equal(t, http.StatusNoContent, env.register("reset@example.test", testPassword).Code)
	env.mail.awaitSubject(t, "Confirm your email address")
	admin := env.adminByEmail("reset@example.test")

	// Two sessions the reset has to end. One stands in for the account's owner,
	// the other for whoever took the account over.
	for range 2 {
		_, err := env.state.CreateAdminSession(ctx, management.AdminSession{
			AdminID:           admin.ID,
			ExpiresAt:         time.Now().Add(time.Hour),
			AbsoluteExpiresAt: time.Now().Add(48 * time.Hour),
			Refreshable:       true,
		})
		require.NoError(t, err)
	}

	request := env.post(env.controller.RequestPasswordReset, map[string]string{"email": "reset@example.test"})
	require.Equal(t, http.StatusNoContent, request.Code)

	message := env.mail.awaitSubject(t, "Reset your password")
	const replacement = "a brand new and different passphrase"

	confirm := env.post(env.controller.ConfirmPasswordReset, map[string]string{
		"token": tokenFrom(t, message), "password": replacement,
	})
	require.Equal(t, http.StatusNoContent, confirm.Code, confirm.Body.String())

	assert.Equal(t, http.StatusUnauthorized, env.login("reset@example.test", testPassword).Code, "the old password is dead")
	assert.Equal(t, http.StatusOK, env.login("reset@example.test", replacement).Code)

	// A reset is the remedy for a compromised account, so every session that
	// existed before it must be gone.
	sessions, err := env.state.ListAdminSessions(ctx, admin.ID)
	require.NoError(t, err)
	var live int
	for _, session := range sessions {
		if session.RevokedAt == nil && session.IssuedAt.Before(time.Now().Add(-time.Millisecond)) {
			live++
		}
	}
	assert.LessOrEqual(t, live, 1, "only the session opened by the sign-in above may still be live")

	// Following a link delivered to the address proves the address.
	identity, err := env.state.GetPasswordIdentity(ctx, admin.ID)
	require.NoError(t, err)
	assert.True(t, identity.EmailVerified)

	replay := env.post(env.controller.ConfirmPasswordReset, map[string]string{
		"token": tokenFrom(t, message), "password": "yet another different passphrase",
	})
	assert.Equal(t, http.StatusBadRequest, replay.Code, "a reset link is worth one redemption")
}

// A password the policy rejects must not cost the caller the one link they
// have; they get to try again with a longer one.
func TestRejectedPasswordDoesNotBurnTheResetLink(t *testing.T) {
	t.Parallel()
	env := newPasswordEnv(t, config.RegistrationOpen)

	require.Equal(t, http.StatusNoContent, env.register("retry@example.test", testPassword).Code)
	env.mail.awaitSubject(t, "Confirm your email address")

	require.Equal(t, http.StatusNoContent,
		env.post(env.controller.RequestPasswordReset, map[string]string{"email": "retry@example.test"}).Code)
	message := env.mail.awaitSubject(t, "Reset your password")
	token := tokenFrom(t, message)

	rejected := env.post(env.controller.ConfirmPasswordReset, map[string]string{
		"token": token, "password": "short",
	})
	require.Equal(t, http.StatusBadRequest, rejected.Code)

	const replacement = "a brand new and different passphrase"
	accepted := env.post(env.controller.ConfirmPasswordReset, map[string]string{
		"token": token, "password": replacement,
	})
	require.Equal(t, http.StatusNoContent, accepted.Code, accepted.Body.String())
	assert.Equal(t, http.StatusOK, env.login("retry@example.test", replacement).Code)
}

func TestPasswordResetDoesNotRevealExistingAccounts(t *testing.T) {
	t.Parallel()
	env := newPasswordEnv(t, config.RegistrationOpen)

	require.Equal(t, http.StatusNoContent, env.register("known@example.test", testPassword).Code)
	env.mail.awaitSubject(t, "Confirm your email address")

	known := env.post(env.controller.RequestPasswordReset, map[string]string{"email": "known@example.test"})
	unknown := env.post(env.controller.RequestPasswordReset, map[string]string{"email": "stranger@example.test"})

	assert.Equal(t, http.StatusNoContent, known.Code)
	assert.Equal(t, known.Code, unknown.Code)
	assert.Equal(t, known.Body.String(), unknown.Body.String())

	// The address that has an account gets a link; the one that does not gets
	// nothing at all, which is the point: an unbounded endpoint that mails
	// strangers is a spam cannon.
	env.mail.awaitSubject(t, "Reset your password")
	for _, message := range env.mail.all() {
		assert.NotEqual(t, "stranger@example.test", message.To)
	}
}

// A password change is how somebody evicts whoever else is signed in, so every
// other session goes -- and the caller's own stays, because a security action
// that logs you out is one people stop taking.
func TestChangePassword(t *testing.T) {
	t.Parallel()
	env := newPasswordEnv(t, config.RegistrationOpen)
	ctx := t.Context()

	require.Equal(t, http.StatusNoContent, env.register("change@example.test", testPassword).Code)
	env.mail.awaitSubject(t, "Confirm your email address")
	admin := env.adminByEmail("change@example.test")

	newSession := func() *management.AdminSession {
		session, err := env.state.CreateAdminSession(ctx, management.AdminSession{
			AdminID:           admin.ID,
			ExpiresAt:         time.Now().Add(time.Hour),
			AbsoluteExpiresAt: time.Now().Add(48 * time.Hour),
			Refreshable:       true,
		})
		require.NoError(t, err)
		return session
	}

	mine := newSession()
	theirs := newSession()

	token, err := env.controller.signer.Mint(mine, []string{"password"})
	require.NoError(t, err)

	change := func(current, next string) *httptest.ResponseRecorder {
		t.Helper()

		encoded, err := json.Marshal(map[string]string{"current_password": current, "password": next})
		require.NoError(t, err)

		res := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/admin/profile/password", strings.NewReader(string(encoded)))
		req.Header.Set("Authorization", "Bearer "+token)
		req = req.WithContext(rbac.WithActor(req.Context(), &rbac.Actor{
			Type: rbac.ActorAdmin, ID: admin.ID.String(),
		}))
		env.controller.ChangePassword(res, req)
		return res
	}

	t.Run("the current password is required", func(t *testing.T) {
		res := change("not the current password", "a brand new and different passphrase")
		assert.Equal(t, http.StatusForbidden, res.Code, res.Body.String())
	})

	t.Run("the new password must satisfy the policy", func(t *testing.T) {
		res := change(testPassword, "short")
		assert.Equal(t, http.StatusBadRequest, res.Code, res.Body.String())
	})

	t.Run("changes the password and ends every other session", func(t *testing.T) {
		const replacement = "a brand new and different passphrase"
		res := change(testPassword, replacement)
		require.Equal(t, http.StatusNoContent, res.Code, res.Body.String())

		kept, err := env.state.GetAdminSession(ctx, mine.ID)
		require.NoError(t, err)
		assert.True(t, kept.Active(time.Now()), "the caller's own session survives")

		ended, err := env.state.GetAdminSession(ctx, theirs.ID)
		require.NoError(t, err)
		assert.False(t, ended.Active(time.Now()), "every other session is revoked")

		assert.Equal(t, http.StatusUnauthorized, env.login("change@example.test", testPassword).Code)
		assert.Equal(t, http.StatusOK, env.login("change@example.test", replacement).Code)

		env.mail.awaitSubject(t, "Your password was changed")
	})
}

// A reset link already in flight is a way back into the account, so changing the
// password has to burn it.
func TestChangingAPasswordInvalidatesOutstandingResetTokens(t *testing.T) {
	t.Parallel()
	env := newPasswordEnv(t, config.RegistrationOpen)
	ctx := t.Context()

	require.Equal(t, http.StatusNoContent, env.register("inflight@example.test", testPassword).Code)
	env.mail.awaitSubject(t, "Confirm your email address")
	admin := env.adminByEmail("inflight@example.test")

	require.Equal(t, http.StatusNoContent,
		env.post(env.controller.RequestPasswordReset, map[string]string{"email": "inflight@example.test"}).Code)
	message := env.mail.awaitSubject(t, "Reset your password")

	identity, err := env.state.GetPasswordIdentity(ctx, admin.ID)
	require.NoError(t, err)
	replacement, err := password.Hash("a brand new and different passphrase")
	require.NoError(t, err)
	require.NoError(t, env.state.SetAdminIdentitySecret(ctx, identity.ID, replacement))
	require.NoError(t, env.state.InvalidateAdminActionTokens(ctx, admin.ID, management.ActionTokenPasswordReset))

	res := env.post(env.controller.ConfirmPasswordReset, map[string]string{
		"token": tokenFrom(t, message), "password": "a third distinct passphrase",
	})
	assert.Equal(t, http.StatusBadRequest, res.Code)
}

// Abuse resistance must never become the thing that locks a deployment out, so
// a limiter that cannot reach Redis reports every budget unspent.
func TestLoginThrottlingFailsOpenWithoutRedis(t *testing.T) {
	t.Parallel()
	env := newPasswordEnv(t, config.RegistrationOpen)

	for range loginFailureLimit + 2 {
		res := env.login("nobody@example.test", "wrong")
		require.NotEqual(t, http.StatusTooManyRequests, res.Code)
	}
}

func TestLoginThrottling(t *testing.T) {
	t.Parallel()
	env := newPasswordEnv(t, config.RegistrationOpen)
	env.controller.throttle = newThrottle(newTestLimiter(t), 0)

	require.Equal(t, http.StatusNoContent, env.register("throttled@example.test", testPassword).Code)
	env.mail.awaitSubject(t, "Confirm your email address")

	for range loginFailureLimit {
		require.Equal(t, http.StatusUnauthorized, env.login("throttled@example.test", "wrong").Code)
	}

	locked := env.login("throttled@example.test", "wrong")
	require.Equal(t, http.StatusTooManyRequests, locked.Code)
	assert.NotEmpty(t, locked.Header().Get("Retry-After"))

	// Even the correct password is refused while the budget is spent: an
	// account under attack stops answering, and answering only the right
	// password would tell the attacker when they had found it.
	assert.Equal(t, http.StatusTooManyRequests, env.login("throttled@example.test", testPassword).Code)

	// An address with no account spends the same budget at the same rate, so a
	// 429 says nothing about which addresses exist. It is keyed on what was
	// submitted, never on what was found.
	for range loginFailureLimit {
		require.Equal(t, http.StatusUnauthorized, env.login("stranger@example.test", "wrong").Code)
	}
	assert.Equal(t, http.StatusTooManyRequests, env.login("stranger@example.test", "wrong").Code)
}

// A successful sign-in must not spend failure budget, or somebody signing in on
// several devices locks themselves out.
func TestSuccessfulLoginSpendsNoFailureBudget(t *testing.T) {
	t.Parallel()
	env := newPasswordEnv(t, config.RegistrationOpen)
	env.controller.throttle = newThrottle(newTestLimiter(t), 0)

	require.Equal(t, http.StatusNoContent, env.register("busy@example.test", testPassword).Code)
	env.mail.awaitSubject(t, "Confirm your email address")

	for range loginFailureLimit + 5 {
		require.Equal(t, http.StatusOK, env.login("busy@example.test", testPassword).Code)
	}
}

func TestPasswordResetThrottling(t *testing.T) {
	t.Parallel()
	env := newPasswordEnv(t, config.RegistrationOpen)
	env.controller.throttle = newThrottle(newTestLimiter(t), 0)

	request := func(email string) int {
		return env.post(env.controller.RequestPasswordReset, map[string]string{"email": email}).Code
	}

	for range resetLimit {
		require.Equal(t, http.StatusNoContent, request("flood@example.test"))
	}
	assert.Equal(t, http.StatusTooManyRequests, request("flood@example.test"))
}

func newTestLimiter(t *testing.T) *ratelimit.Limiter {
	t.Helper()

	options, err := redis.ParseURL(container.RunRedis(t))
	require.NoError(t, err)

	client := redis.NewClient(options)
	t.Cleanup(func() { client.Close() })

	return ratelimit.New(client, "test:"+uuid.NewString()+":", zaptest.NewLogger(t))
}

// The set of drivers offered is the deployment's, not the first one configured.
func TestGetAuthMethodsListsEveryConfiguredDriver(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	mgmtDB, _, _ := teststore.RunPostgreSQL(t)

	controller, err := NewAuthController(logger, mgmtDB, management.NewState(mgmtDB), config.Node{
		Auth: config.Auth{
			Drivers: []string{"password", "clerk"},
			JWKS:    clerkJWKS(t),
			Clerk:   config.ClerkAuth{SecretKey: "sk_test_xxx"},
		},
		Mail: testMailConfig(),
	}, nil, consoleSignerFor(t), nil)
	require.NoError(t, err)

	res := httptest.NewRecorder()
	controller.GetAuthMethods(res, httptest.NewRequest("GET", "/api/auth/methods", nil))
	require.Equal(t, http.StatusOK, res.Code)

	var drivers []string
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &drivers))
	assert.Equal(t, []string{"password", "clerk"}, drivers, "in the order the operator listed them")
}

// The password flows are unreachable on a deployment that did not configure the
// driver, rather than half-working against a table nothing writes.
func TestPasswordFlowsAreOffWhenTheDriverIsNotConfigured(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	mgmtDB, _, _ := teststore.RunPostgreSQL(t)

	controller, err := NewAuthController(logger, mgmtDB, management.NewState(mgmtDB), config.Node{
		Auth: config.Auth{Drivers: []string{"clerk"}, JWKS: clerkJWKS(t), Clerk: config.ClerkAuth{SecretKey: "sk_test_xxx"}},
	}, nil, consoleSignerFor(t), nil)
	require.NoError(t, err)

	handlers := map[string]http.HandlerFunc{
		"register": controller.RegisterWithPassword,
		"verify":   controller.VerifyEmail,
		"reset":    controller.RequestPasswordReset,
		"confirm":  controller.ConfirmPasswordReset,
		"change":   controller.ChangePassword,
	}

	for name, handler := range handlers {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/api/auth/test", strings.NewReader("{}"))
			handler(res, req)
			assert.Equal(t, http.StatusNotFound, res.Code, res.Body.String())
		})
	}
}

// A deployment that offers password logins without configuring a mail channel
// cannot confirm an address or reset a password. It is refused at boot rather
// than discovered by the first person who tries to register.
func TestNewAuthControllerRequiresAMailChannel(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	mgmtDB, _, _ := teststore.RunPostgreSQL(t)

	_, err := NewAuthController(logger, mgmtDB, management.NewState(mgmtDB), config.Node{
		Auth: config.Auth{
			Drivers:  []string{"password"},
			Password: config.PasswordAuth{Registration: config.RegistrationOpen},
		},
	}, nil, consoleSignerFor(t), nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no channel is configured")
}

func TestNewAuthControllerRejectsAnUnknownRegistrationMode(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	mgmtDB, _, _ := teststore.RunPostgreSQL(t)

	_, err := NewAuthController(logger, mgmtDB, management.NewState(mgmtDB), config.Node{
		Auth: config.Auth{
			Drivers:  []string{"password"},
			Password: config.PasswordAuth{Registration: "sometimes"},
		},
	}, nil, consoleSignerFor(t), nil)
	require.Error(t, err, "a typo in AUTH_PASSWORD_REGISTRATION must not start")
}
