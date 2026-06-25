package v1

import (
	"net/http"
	"time"

	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/http/controllers/v1/client/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/rbac"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"go.uber.org/zap"
)

func NewSessionsController(client *ClientController, signer *auth.SessionSigner) *SessionsController {
	return &SessionsController{client: client, signer: signer}
}

type SessionsController struct {
	client *ClientController
	signer *auth.SessionSigner
}

// CreateSession mints a short-lived session token for an end user under a
// session policy. It is a privileged backend operation: only an authorized API
// key may call it (API keys are private/backend-only), and the session auth
// method named in the path must belong to the project named in the URL. The
// session's permissions come from the policy.
//
// The URL projectID is authoritative: the auth layer already bound the API key
// to it, so actor.ProjectID equals projectID here, but the session method is
// validated against the URL project directly.
func (srv *SessionsController) CreateSession(w http.ResponseWriter, r *http.Request, projectID oapi.ProjectID, authMethodID openapi_types.UUID) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)

	if actor == nil || actor.Type != rbac.ActorAPIKey {
		oapi.WriteProblem(w, problem.ErrForbidden(problem.Describe("an API key is required to mint sessions")))
		return
	}

	if srv.signer == nil {
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("session signing is not configured")))
		return
	}

	var body oapi.CreateSessionJSONRequestBody
	if err := json.Decode(r.Body, &body); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	// The policy must exist, be a session method, and live in the URL project.
	method, err := srv.client.mgmt.GetSessionAuthMethod(authMethodID)
	if err != nil || method.ProjectID != projectID {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("session auth method not found")))
		return
	}

	ttl := time.Duration(method.TTLSeconds) * time.Second
	token, expiresAt, err := srv.signer.Mint(method.ID, body.UserId, ttl)
	if err != nil {
		srv.client.logger.Error("failed to mint session", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	json.Write(w, http.StatusCreated, oapi.SessionToken{Token: token, ExpiresAt: expiresAt})
}
