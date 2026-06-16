package v1

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/client/oapi"
	"github.com/lunogram/platform/internal/http/json"
	httpparams "github.com/lunogram/platform/internal/http/params"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/lunogram/platform/pkg/modules"
	"go.uber.org/zap"
)

type InboxController struct {
	*ClientController
}

func NewInboxController(client *ClientController) *InboxController {
	return &InboxController{ClientController: client}
}

func (srv *InboxController) PostUserInboxMessages(w http.ResponseWriter, r *http.Request) {
	projectID, err := srv.engine.AllowedProject(r.Context(), "inbox", rbac.Create)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var req oapi.PostUserInboxMessagesRequest
	if err := json.Decode(r.Body, &req); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	if len(req) > 100 {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("batch size exceeds maximum of 100")))
		return
	}

	for _, item := range req {
		msg, err := item.ToInboxMessage(projectID)
		if err != nil {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid message data")))
			return
		}
		if err := srv.pubsub.Publish(r.Context(), schemas.UserInboxProcess(projectID), msg); err != nil {
			srv.logger.Error("failed to publish user inbox message", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Debug("accepted user inbox messages", zap.Int("count", len(req)))

	w.WriteHeader(http.StatusAccepted)
}

func (srv *InboxController) GetUserInbox(w http.ResponseWriter, r *http.Request, params oapi.GetUserInboxParams) {
	projectID, err := srv.engine.AllowedProject(r.Context(), "inbox", rbac.Read)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	userID, err := srv.users.LookupUserID(r.Context(), projectID, []subjects.ExternalIDParam{{Source: params.Source, ExternalID: params.ExternalId}})
	if errors.Is(err, subjects.ErrUserNotFound) {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}
	if err != nil {
		srv.logger.Error("failed to lookup user", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	filter := clientInboxFilter(params.Status, params.Tags, params.MessageSource, params.Priority, modules.Channel(params.Channel))
	messages, total, err := srv.users.ListUserInboxMessages(r.Context(), projectID, userID, store.Pagination{Limit: params.Limit.ToInt(), Offset: params.Offset.ToInt()}, filter)
	if err != nil {
		srv.logger.Error("failed to list user inbox messages", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	json.Write(w, http.StatusOK, oapi.InboxMessageList{
		Results: inboxMessagesToClientOAPI(messages),
		Total:   total,
		Limit:   params.Limit.ToInt(),
		Offset:  params.Offset.ToInt(),
	})
}

func (srv *InboxController) GetUserInboxCount(w http.ResponseWriter, r *http.Request, params oapi.GetUserInboxCountParams) {
	projectID, err := srv.engine.AllowedProject(r.Context(), "inbox", rbac.Read)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	userID, err := srv.users.LookupUserID(r.Context(), projectID, []subjects.ExternalIDParam{{Source: params.Source, ExternalID: params.ExternalId}})
	if errors.Is(err, subjects.ErrUserNotFound) {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}
	if err != nil {
		srv.logger.Error("failed to lookup user", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	counts, err := srv.users.CountUserInboxMessages(r.Context(), projectID, userID, string(params.Channel))
	if err != nil {
		srv.logger.Error("failed to count user inbox messages", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	json.Write(w, http.StatusOK, oapi.InboxCount{Unread: counts.Unread, Total: counts.Total})
}

func (srv *InboxController) PostUserInboxRead(w http.ResponseWriter, r *http.Request) {
	srv.publishUserInboxStateEvents(w, r, schemas.UserInboxRead, "read")
}

func (srv *InboxController) PostUserInboxArchived(w http.ResponseWriter, r *http.Request) {
	srv.publishUserInboxStateEvents(w, r, schemas.UserInboxArchived, "archived")
}

func (srv *InboxController) PostOrganizationInboxMessages(w http.ResponseWriter, r *http.Request) {
	projectID, err := srv.engine.AllowedProject(r.Context(), "inbox", rbac.Create)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var req oapi.PostOrganizationInboxMessagesRequest
	if err := json.Decode(r.Body, &req); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	if len(req) > 100 {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("batch size exceeds maximum of 100")))
		return
	}

	for _, item := range req {
		msg, err := item.ToInboxMessage(projectID)
		if err != nil {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid message data")))
			return
		}
		if err := srv.pubsub.Publish(r.Context(), schemas.OrganizationInboxProcess(projectID), msg); err != nil {
			srv.logger.Error("failed to publish organization inbox message", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Debug("accepted organization inbox messages", zap.Int("count", len(req)))

	w.WriteHeader(http.StatusAccepted)
}

func (srv *InboxController) GetOrganizationInbox(w http.ResponseWriter, r *http.Request, params oapi.GetOrganizationInboxParams) {
	projectID, err := srv.engine.AllowedProject(r.Context(), "inbox", rbac.Read)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	organizationID, err := srv.users.LookupOrganizationID(r.Context(), projectID, []subjects.ExternalIDParam{{Source: params.Source, ExternalID: params.ExternalId}})
	if errors.Is(err, subjects.ErrOrgNotFound) {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}
	if err != nil {
		srv.logger.Error("failed to lookup organization", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	filter := clientInboxFilter(params.Status, params.Tags, params.MessageSource, params.Priority, modules.Channel(params.Channel))
	messages, total, err := srv.users.ListOrganizationInboxMessages(r.Context(), projectID, organizationID, store.Pagination{Limit: params.Limit.ToInt(), Offset: params.Offset.ToInt()}, filter)
	if err != nil {
		srv.logger.Error("failed to list organization inbox messages", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	json.Write(w, http.StatusOK, oapi.InboxMessageList{
		Results: inboxMessagesToClientOAPI(messages),
		Total:   total,
		Limit:   params.Limit.ToInt(),
		Offset:  params.Offset.ToInt(),
	})
}

func (srv *InboxController) GetOrganizationInboxCount(w http.ResponseWriter, r *http.Request, params oapi.GetOrganizationInboxCountParams) {
	projectID, err := srv.engine.AllowedProject(r.Context(), "inbox", rbac.Read)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	organizationID, err := srv.users.LookupOrganizationID(r.Context(), projectID, []subjects.ExternalIDParam{{Source: params.Source, ExternalID: params.ExternalId}})
	if errors.Is(err, subjects.ErrOrgNotFound) {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("organization not found")))
		return
	}
	if err != nil {
		srv.logger.Error("failed to lookup organization", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	counts, err := srv.users.CountOrganizationInboxMessages(r.Context(), projectID, organizationID, string(params.Channel))
	if err != nil {
		srv.logger.Error("failed to count organization inbox messages", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	json.Write(w, http.StatusOK, oapi.InboxCount{Unread: counts.Unread, Total: counts.Total})
}

func (srv *InboxController) PostOrganizationInboxRead(w http.ResponseWriter, r *http.Request) {
	srv.publishOrganizationInboxStateEvents(w, r, schemas.OrganizationInboxRead, "read")
}

func (srv *InboxController) PostOrganizationInboxArchived(w http.ResponseWriter, r *http.Request) {
	srv.publishOrganizationInboxStateEvents(w, r, schemas.OrganizationInboxArchived, "archived")
}

// publishUserInboxStateEvents decodes a bulk array of user inbox state events
// and publishes one pubsub message per item onto the subject returned by
// subjectFor (e.g. UserInboxRead, UserInboxArchived). Each lifecycle action
// has its own subject so it can be backed by its own consumer, retry policy,
// and broker-side filter. action is used for logging context.
func (srv *InboxController) publishUserInboxStateEvents(w http.ResponseWriter, r *http.Request, subjectFor func(uuid.UUID) schemas.Subject, action string) {
	projectID, err := srv.engine.AllowedProject(r.Context(), "inbox", rbac.Update)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var req oapi.UserInboxMessageEvents
	if err := json.Decode(r.Body, &req); err != nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	subject := subjectFor(projectID)
	for _, item := range req {
		msg := schemas.InboxStateEvent{
			ProjectID:   projectID,
			MessageID:   item.MessageId,
			Identifiers: oapi.ToParams(item.Target),
		}
		if err := srv.pubsub.Publish(r.Context(), subject, msg); err != nil {
			srv.logger.Error("failed to publish user inbox event", zap.String("action", action), zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}
	}

	w.WriteHeader(http.StatusAccepted)
}

// publishOrganizationInboxStateEvents mirrors publishUserInboxStateEvents for
// the organization-scoped inbox.
func (srv *InboxController) publishOrganizationInboxStateEvents(w http.ResponseWriter, r *http.Request, subjectFor func(uuid.UUID) schemas.Subject, action string) {
	projectID, err := srv.engine.AllowedProject(r.Context(), "inbox", rbac.Update)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var req oapi.OrganizationInboxMessageEvents
	if err := json.Decode(r.Body, &req); err != nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	subject := subjectFor(projectID)
	for _, item := range req {
		msg := schemas.InboxStateEvent{
			ProjectID:   projectID,
			MessageID:   item.MessageId,
			Identifiers: oapi.ToParams(item.Target),
		}
		if err := srv.pubsub.Publish(r.Context(), subject, msg); err != nil {
			srv.logger.Error("failed to publish organization inbox event", zap.String("action", action), zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}
	}

	w.WriteHeader(http.StatusAccepted)
}

func clientInboxFilter[S ~string](status *S, tags *string, source *string, priority *int, channel modules.Channel) subjects.InboxListFilter {
	filter := subjects.InboxListFilter{MessageSource: source, Priority: priority, Channel: channel}
	if status != nil {
		filter.Status = string(*status)
	}
	if tags != nil && *tags != "" {
		filter.Tags = httpparams.Split(*tags)
	}
	return filter
}

func inboxMessagesToClientOAPI(messages subjects.InboxMessages) []oapi.InboxMessage {
	results := make([]oapi.InboxMessage, len(messages))
	for i := range messages {
		results[i] = inboxMessageToClientOAPI(messages[i])
	}
	return results
}

func inboxMessageToClientOAPI(message subjects.InboxMessage) oapi.InboxMessage {
	return oapi.InboxMessage{
		Id:               message.ID,
		ProjectId:        message.ProjectID,
		UserId:           message.UserID,
		OrganizationId:   message.OrganizationID,
		ExternalId:       message.ExternalID,
		Channel:          oapi.Channel(message.Channel),
		SenderIdentityId: message.SenderIdentityID,
		CampaignId:       message.CampaignID,
		BroadcastId:      message.BroadcastID,
		Content:          message.Content,
		Data:             message.Data,
		Tags:             []string(message.Tags),
		Priority:         message.Priority,
		Source:           message.Source,
		ScheduledAt:      message.ScheduledAt,
		ExpiresAt:        message.ExpiresAt,
		ReadAt:           message.ReadAt,
		ArchivedAt:       message.ArchivedAt,
		CreatedAt:        message.CreatedAt,
		UpdatedAt:        message.UpdatedAt,
	}
}
