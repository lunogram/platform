package auth

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sessionMgmt builds a DB-backed management.State for the session middleware
// tests (GetSessionAuthMethod reads from it).
func sessionMgmt(t *testing.T) *management.State {
	t.Helper()
	mgmtDB, _, _ := teststore.RunPostgreSQL(t)
	return management.NewState(mgmtDB)
}

// newSessionMethod inserts a session auth method with the given subject scope
// and returns the created method together with its org/project ids (the write
// AuthMethod does not carry the organization id).
func newSessionMethod(t *testing.T, mgmt *management.State, scope management.SubjectScope) (method *management.AuthMethod, orgID, projectID uuid.UUID) {
	t.Helper()
	orgID, err := mgmt.OrganizationsStore.CreateOrganization(t.Context(), "Session Org")
	require.NoError(t, err)
	projectID, err = mgmt.ProjectsStore.CreateProject(t.Context(), management.Project{
		OrganizationID: &orgID,
		Name:           "Session Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	method, err = mgmt.CreateAuthMethod(t.Context(), projectID, management.CreateAuthMethodInput{
		Type:         management.MethodTypeSession,
		Name:         "session policy",
		Role:         "client",
		SubjectScope: scope,
		Session:      &management.Session{TTLSeconds: 3600},
	})
	require.NoError(t, err)
	return method, orgID, projectID
}

func TestWithSession(t *testing.T) {
	t.Parallel()

	t.Run("nil signer declines (sessions disabled)", func(t *testing.T) {
		t.Parallel()
		handler := WithSession(nil, nil)
		token, _, err := testSigner(t, "").Mint(uuid.New(), "user_1", time.Hour)
		require.NoError(t, err)

		_, err = handler(context.Background(), token)
		assert.ErrorIs(t, err, ErrUnauthorized)
	})

	t.Run("empty token declines", func(t *testing.T) {
		t.Parallel()
		handler := WithSession(nil, testSigner(t, ""))
		_, err := handler(context.Background(), "")
		assert.ErrorIs(t, err, ErrUnauthorized)
	})

	t.Run("a token signed by a different key is rejected", func(t *testing.T) {
		t.Parallel()
		token, _, err := testSigner(t, "").Mint(uuid.New(), "user_1", time.Hour)
		require.NoError(t, err)

		other := testSigner(t, "")
		_, err = WithSession(nil, other)(context.Background(), token)
		assert.ErrorIs(t, err, ErrUnauthorized)
	})

	// A token whose signature does not verify (wrong key, wrong alg, malformed)
	// stays a generic ErrUnauthorized so the auth chain falls through to the next
	// handler. Only once the signature proves the token is authentically ours does
	// the handler surface a precise, debuggable reason for the remaining failures.

	t.Run("an expired token surfaces a debuggable reason", func(t *testing.T) {
		t.Parallel()
		signer := testSigner(t, "")
		token, _, err := signer.Mint(uuid.New(), "user_1", -time.Hour)
		require.NoError(t, err)

		_, err = WithSession(nil, signer)(context.Background(), token)
		assert.NotErrorIs(t, err, ErrUnauthorized)
		assert.ErrorContains(t, err, "expired")
	})

	t.Run("a missing exp claim surfaces a debuggable reason", func(t *testing.T) {
		t.Parallel()
		signer := testSigner(t, "")
		token := signES256(t, signer, jwt.MapClaims{
			"iss":              signer.issuer,
			"sub":              "user_1",
			sessionMethodClaim: uuid.New().String(),
			// no "exp": WithExpirationRequired rejects it
		})
		_, err := WithSession(nil, signer)(context.Background(), token)
		assert.NotErrorIs(t, err, ErrUnauthorized)
		assert.ErrorContains(t, err, `"exp"`)
	})

	t.Run("a wrong issuer surfaces a debuggable reason", func(t *testing.T) {
		t.Parallel()
		// Signer expects the default issuer; mint with a different one. The
		// signature still verifies (same key), so this is one of our tokens.
		minter := testSigner(t, "https://evil.example")
		token, _, err := minter.Mint(uuid.New(), "user_1", time.Hour)
		require.NoError(t, err)

		verifier := &SessionSigner{key: minter.key, issuer: defaultSessionIssuer}
		_, err = WithSession(nil, verifier)(context.Background(), token)
		assert.NotErrorIs(t, err, ErrUnauthorized)
		assert.ErrorContains(t, err, "issuer")
	})

	t.Run("a token signed by a different key stays generic (chain falls through)", func(t *testing.T) {
		t.Parallel()
		// A token minted by a foreign key (e.g. another scheme's) must not be
		// described as a session token; it stays ErrUnauthorized so the next
		// handler gets a turn.
		minter := testSigner(t, "")
		token, _, err := minter.Mint(uuid.New(), "user_1", time.Hour)
		require.NoError(t, err)

		_, err = WithSession(nil, testSigner(t, ""))(context.Background(), token)
		assert.ErrorIs(t, err, ErrUnauthorized)
	})

	t.Run("an unparseable method id claim surfaces a debuggable reason", func(t *testing.T) {
		t.Parallel()
		signer := testSigner(t, "")
		token := signES256(t, signer, jwt.MapClaims{
			"iss":              signer.issuer,
			"sub":              "user_1",
			sessionMethodClaim: "not-a-uuid",
			"exp":              time.Now().Add(time.Hour).Unix(),
		})
		_, err := WithSession(nil, signer)(context.Background(), token)
		assert.NotErrorIs(t, err, ErrUnauthorized)
		assert.ErrorContains(t, err, "amid")
	})

	t.Run("a missing method id claim surfaces a debuggable reason", func(t *testing.T) {
		t.Parallel()
		signer := testSigner(t, "")
		token := signES256(t, signer, jwt.MapClaims{
			"iss": signer.issuer,
			"sub": "user_1",
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		_, err := WithSession(nil, signer)(context.Background(), token)
		assert.NotErrorIs(t, err, ErrUnauthorized)
		assert.ErrorContains(t, err, "amid")
	})

	t.Run("an empty subject surfaces a debuggable reason", func(t *testing.T) {
		t.Parallel()
		signer := testSigner(t, "")
		token := signES256(t, signer, jwt.MapClaims{
			"iss":              signer.issuer,
			"sub":              "",
			sessionMethodClaim: uuid.New().String(),
			"exp":              time.Now().Add(time.Hour).Unix(),
		})
		_, err := WithSession(nil, signer)(context.Background(), token)
		assert.NotErrorIs(t, err, ErrUnauthorized)
		assert.ErrorContains(t, err, `"sub"`)
	})
}

func TestWithSessionUnknownMethod(t *testing.T) {
	t.Parallel()

	mgmt := sessionMgmt(t)
	signer := testSigner(t, "")
	// A well-formed token whose method id resolves to nothing in the store.
	token, _, err := signer.Mint(uuid.New(), "user_1", time.Hour)
	require.NoError(t, err)

	_, err = WithSession(mgmt, signer)(context.Background(), token)
	assert.NotErrorIs(t, err, ErrUnauthorized)
	assert.ErrorContains(t, err, "auth method")
}

func TestWithSessionBuildsActor(t *testing.T) {
	t.Parallel()

	mgmt := sessionMgmt(t)
	method, _, projectID := newSessionMethod(t, mgmt, management.SubjectScopeOwn)
	signer := testSigner(t, "")

	token, _, err := signer.Mint(method.ID, "user_42", time.Hour)
	require.NoError(t, err)

	// The session is presented on its own project's URL (the post-#262 contract).
	ctx, err := WithSession(mgmt, signer)(clientRequestCtx(projectID.String()), token)
	require.NoError(t, err)

	actor := rbac.FromContext(ctx)
	require.NotNil(t, actor)
	assert.Equal(t, rbac.ActorEndUser, actor.Type)
	assert.Equal(t, method.ID.String(), actor.ID)
	assert.Equal(t, projectID, actor.ProjectID)
	assert.Equal(t, "user_42", actor.Subject)
	assert.Equal(t, SessionSubjectSource(method.ID), actor.SubjectSource)
	assert.Equal(t, rbac.DataScopeOwn, actor.Scope)
}

func TestWithSessionScopeAll(t *testing.T) {
	t.Parallel()

	mgmt := sessionMgmt(t)
	method, orgID, projectID := newSessionMethod(t, mgmt, management.SubjectScopeAll)
	signer := testSigner(t, "")

	token, _, err := signer.Mint(method.ID, "user_7", time.Hour)
	require.NoError(t, err)

	ctx, err := WithSession(mgmt, signer)(clientRequestCtx(projectID.String()), token)
	require.NoError(t, err)

	actor := rbac.FromContext(ctx)
	require.NotNil(t, actor)
	assert.Equal(t, rbac.DataScopeAll, actor.Scope)
	assert.Equal(t, orgID, actor.OrganizationID)
}
