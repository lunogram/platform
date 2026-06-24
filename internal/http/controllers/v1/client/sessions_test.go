package v1

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testSessionSigner builds a SessionSigner backed by a fresh EC P-256 key. The
// client controller is constructed with a nil signer (see setupClientController);
// CreateSession authz tests swap in a real one via withSessionSigner.
func testSessionSigner(t *testing.T) *auth.SessionSigner {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der}))

	signer, err := auth.NewSessionSigner(pemKey, "")
	require.NoError(t, err)
	require.NotNil(t, signer)
	return signer
}

// withSessionSigner rebuilds the embedded SessionsController on top of the same
// ClientController with the given signer. setupClientController wires a nil
// signer; this lets the authz tests exercise the configured-signer paths.
func (tc *testClientController) withSessionSigner(signer *auth.SessionSigner) {
	tc.SessionsController = NewSessionsController(tc.UsersController.ClientController, signer)
}

// createSessionMethod inserts a session auth method in the given project and
// returns its id (the policy CreateSession mints under).
func (tc *testClientController) createSessionMethod(t *testing.T, projectID uuid.UUID) uuid.UUID {
	t.Helper()
	method, err := tc.mgmt.CreateAuthMethod(t.Context(), projectID, management.CreateAuthMethodInput{
		Type:    management.MethodTypeSession,
		Name:    "session policy",
		Role:    "client",
		Session: &management.Session{TTLSeconds: 3600},
	})
	require.NoError(t, err)
	return method.ID
}

func TestCreateSession(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)
	controller.withSessionSigner(testSessionSigner(t))

	orgID, err := controller.mgmt.OrganizationsStore.CreateOrganization(t.Context(), "Test Org")
	require.NoError(t, err)
	projectID, err := controller.mgmt.ProjectsStore.CreateProject(t.Context(), management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	methodID := controller.createSessionMethod(t, projectID)

	body, err := json.Marshal(map[string]any{"user_id": "user_123"})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	actor := rbac.NewActor(rbac.ActorAPIKey, uuid.New().String(),
		rbac.WithOrganizationID(orgID),
		rbac.WithProjectID(projectID),
	)
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
	w := httptest.NewRecorder()

	// The URL names the method's own project, so the mint is authorized.
	controller.CreateSession(w, req, projectID, methodID)

	require.Equal(t, 201, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["token"])
	assert.NotEmpty(t, resp["expires_at"])
}

func TestCreateSessionNonAPIKeyActorForbidden(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)
	controller.withSessionSigner(testSessionSigner(t))

	body, err := json.Marshal(map[string]any{"user_id": "user_123"})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// An end-user actor (e.g. a session) must not be able to mint further sessions.
	actor := rbac.NewActor(rbac.ActorEndUser, uuid.New().String(),
		rbac.WithProjectID(uuid.New()),
	)
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
	w := httptest.NewRecorder()

	controller.CreateSession(w, req, uuid.New(), uuid.New())

	assert.Equal(t, 403, w.Code)
}

func TestCreateSessionMissingActorForbidden(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)
	controller.withSessionSigner(testSessionSigner(t))

	body, err := json.Marshal(map[string]any{"user_id": "user_123"})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	controller.CreateSession(w, req, uuid.New(), uuid.New())

	assert.Equal(t, 403, w.Code)
}

func TestCreateSessionMissingSigner(t *testing.T) {
	t.Parallel()

	// setupClientController wires a nil signer; do not swap one in.
	controller := setupClientController(t)

	body, err := json.Marshal(map[string]any{"user_id": "user_123"})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	actor := rbac.NewActor(rbac.ActorAPIKey, uuid.New().String(),
		rbac.WithOrganizationID(uuid.New()),
		rbac.WithProjectID(uuid.New()),
	)
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
	w := httptest.NewRecorder()

	controller.CreateSession(w, req, uuid.New(), uuid.New())

	assert.Equal(t, 500, w.Code)
}

func TestCreateSessionCrossProjectMethodRejected(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)
	controller.withSessionSigner(testSessionSigner(t))

	orgID, err := controller.mgmt.OrganizationsStore.CreateOrganization(t.Context(), "Test Org")
	require.NoError(t, err)

	// The method lives in projectA; the key acts in projectB.
	projectA, err := controller.mgmt.ProjectsStore.CreateProject(t.Context(), management.Project{
		OrganizationID: &orgID,
		Name:           "Project A",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)
	projectB, err := controller.mgmt.ProjectsStore.CreateProject(t.Context(), management.Project{
		OrganizationID: &orgID,
		Name:           "Project B",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	methodID := controller.createSessionMethod(t, projectA)

	body, err := json.Marshal(map[string]any{"user_id": "user_123"})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	actor := rbac.NewActor(rbac.ActorAPIKey, uuid.New().String(),
		rbac.WithOrganizationID(orgID),
		rbac.WithProjectID(projectB),
	)
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
	w := httptest.NewRecorder()

	// The URL names projectB, but the method belongs to projectA.
	controller.CreateSession(w, req, projectB, methodID)

	// A method in another project is reported as not found, never minted.
	assert.Equal(t, 404, w.Code)
}

func TestCreateSessionUnknownMethodNotFound(t *testing.T) {
	t.Parallel()

	controller := setupClientController(t)
	controller.withSessionSigner(testSessionSigner(t))

	orgID, err := controller.mgmt.OrganizationsStore.CreateOrganization(t.Context(), "Test Org")
	require.NoError(t, err)
	projectID, err := controller.mgmt.ProjectsStore.CreateProject(t.Context(), management.Project{
		OrganizationID: &orgID,
		Name:           "Test Project",
		Timezone:       "UTC",
		Locale:         "en",
	})
	require.NoError(t, err)

	body, err := json.Marshal(map[string]any{"user_id": "user_123"})
	require.NoError(t, err)

	req := httptest.NewRequest("POST", "/api/client/sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	actor := rbac.NewActor(rbac.ActorAPIKey, uuid.New().String(),
		rbac.WithOrganizationID(orgID),
		rbac.WithProjectID(projectID),
	)
	req = req.WithContext(rbac.WithActor(req.Context(), actor))
	w := httptest.NewRecorder()

	controller.CreateSession(w, req, uuid.New(), uuid.New())

	assert.Equal(t, 404, w.Code)
}
