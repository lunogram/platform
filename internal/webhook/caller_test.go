package webhook

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/oapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestNewCaller(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)

	t.Run("with default timeout", func(t *testing.T) {
		cfg := config.Webhook{
			ProjectCreatedURL: "http://example.com/webhook",
		}

		caller := NewCaller(logger, cfg)

		assert.Equal(t, "http://example.com/webhook", caller.projectCreatedURL)
		assert.Equal(t, 30*time.Second, caller.projectCreatedTimeout)
		assert.True(t, caller.Enabled())
	})

	t.Run("with custom timeout", func(t *testing.T) {
		cfg := config.Webhook{
			ProjectCreatedURL:     "http://example.com/webhook",
			ProjectCreatedTimeout: 10 * time.Second,
		}

		caller := NewCaller(logger, cfg)

		assert.Equal(t, 10*time.Second, caller.projectCreatedTimeout)
	})

	t.Run("disabled when no URL configured", func(t *testing.T) {
		cfg := config.Webhook{}

		caller := NewCaller(logger, cfg)

		assert.False(t, caller.Enabled())
	})
}

func TestProjectCreated(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	projectID := uuid.New()
	orgID := uuid.New()
	timezone := "America/New_York"
	locale := "en-US"

	project := oapi.ProjectDetails{
		Id:             projectID,
		OrganizationId: orgID,
		Name:           "Test Project",
		Timezone:       &timezone,
		Locale:         &locale,
		CreatedAt:      time.Now().UTC(),
	}

	t.Run("sends webhook successfully and forwards headers", func(t *testing.T) {
		var receivedPayload oapi.ProjectCreatedEvent
		var receivedAuthHeader string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
			assert.Equal(t, "project.created", r.Header.Get("X-Webhook-Event"))

			receivedAuthHeader = r.Header.Get("Authorization")

			err := json.NewDecoder(r.Body).Decode(&receivedPayload)
			require.NoError(t, err)

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}))
		defer server.Close()

		cfg := config.Webhook{
			ProjectCreatedURL: server.URL,
		}
		caller := NewCaller(logger, cfg)

		req := httptest.NewRequest(http.MethodPost, "/projects", nil)
		req.Header.Set("Authorization", "Bearer test-token-123")
		req.Header.Set("X-Custom-Header", "custom-value")

		err := caller.ProjectCreated(req.Context(), req, project)
		require.NoError(t, err)

		assert.Equal(t, oapi.ProjectCreated, receivedPayload.Event)
		assert.Equal(t, projectID, receivedPayload.Project.Id)
		assert.Equal(t, orgID, receivedPayload.Project.OrganizationId)
		assert.Equal(t, "Test Project", receivedPayload.Project.Name)
		assert.Equal(t, "Bearer test-token-123", receivedAuthHeader)
	})

	t.Run("does nothing when no URL configured", func(t *testing.T) {
		cfg := config.Webhook{}
		caller := NewCaller(logger, cfg)

		req := httptest.NewRequest(http.MethodPost, "/projects", nil)

		// This should not panic or make any HTTP calls
		err := caller.ProjectCreated(req.Context(), req, project)
		require.NoError(t, err)
	})

	t.Run("does nothing when caller is nil", func(t *testing.T) {
		var caller *Caller

		req := httptest.NewRequest(http.MethodPost, "/projects", nil)

		// This should not panic or make any HTTP calls
		err := caller.ProjectCreated(req.Context(), req, project)
		require.NoError(t, err)
	})

	t.Run("returns error on server error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		cfg := config.Webhook{
			ProjectCreatedURL: server.URL,
		}
		caller := NewCaller(logger, cfg)

		req := httptest.NewRequest(http.MethodPost, "/projects", nil)

		err := caller.ProjectCreated(req.Context(), req, project)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "webhook returned error status")
	})
}
