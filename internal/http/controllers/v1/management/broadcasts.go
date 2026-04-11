//go:build !enterprise

package v1

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/consumer"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/nats-io/nats.go/jetstream"
	"go.uber.org/zap"
)

func NewBroadcastsController(_ *zap.Logger, _, _ *sqlx.DB, _ pubsub.Publisher, _ jetstream.JetStream, _ *rbac.Engine, _ consumer.Namespace) *BroadcastsController {
	return &BroadcastsController{}
}

type BroadcastsController struct{}

func (srv *BroadcastsController) CreateBroadcast(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	oapi.WriteProblem(w, problem.ErrNotFound())
}

func (srv *BroadcastsController) ListBroadcasts(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListBroadcastsParams) {
	oapi.WriteProblem(w, problem.ErrNotFound())
}

func (srv *BroadcastsController) GetBroadcast(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, broadcastID uuid.UUID) {
	oapi.WriteProblem(w, problem.ErrNotFound())
}

func (srv *BroadcastsController) SendBroadcast(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, broadcastID uuid.UUID) {
	oapi.WriteProblem(w, problem.ErrNotFound())
}

func (srv *BroadcastsController) UpdateBroadcast(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, broadcastID uuid.UUID) {
	oapi.WriteProblem(w, problem.ErrNotFound())
}

func (srv *BroadcastsController) CancelBroadcast(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, broadcastID uuid.UUID) {
	oapi.WriteProblem(w, problem.ErrNotFound())
}

func (srv *BroadcastsController) GetBroadcastUsers(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, broadcastID uuid.UUID, params oapi.GetBroadcastUsersParams) {
	oapi.WriteProblem(w, problem.ErrNotFound())
}

func (srv *BroadcastsController) StreamBroadcastProgress(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, broadcastID uuid.UUID) {
	oapi.WriteProblem(w, problem.ErrNotFound())
}
