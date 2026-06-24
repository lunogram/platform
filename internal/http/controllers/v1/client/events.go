package v1

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/http/controllers/v1/client/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

type EventsController struct {
	*ClientController
}

func NewEventsController(client *ClientController) *EventsController {
	return &EventsController{ClientController: client}
}

func (srv *EventsController) PostUserEvents(w http.ResponseWriter, r *http.Request, _ oapi.ProjectID) {
	projectID, err := srv.engine.AllowedProject(r.Context(), "events", rbac.Create)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	ctx := r.Context()

	var events oapi.PostEventsRequest
	err = json.Decode(r.Body, &events)
	if err != nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Int("events", len(events)))
	logger.Info("posting events")

	names := make([]string, len(events))
	for i, event := range events {
		names[i] = event.Name
	}
	if err := srv.constraints.Enforce(ctx, rbac.ResourceEvents, names); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	ownScoped := auth.OwnDataScoped(ctx)

	for _, event := range events {
		// An own-data actor may only emit events about itself: attribute
		// matching is forbidden and the identifier is always the verified
		// subject, regardless of what the client sent.
		if ownScoped && event.Match != nil {
			oapi.WriteProblem(w, problem.ErrForbidden(problem.Describe("end users may not emit match-based events")))
			return
		}
		if !ownScoped {
			if event.Match != nil && event.Identifier != nil {
				oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("match and identifier are mutually exclusive")))
				return
			}
			if event.Match == nil && event.Identifier == nil {
				oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("one of match or identifier is required")))
				return
			}
		}

		switch {
		case event.Match != nil:
			msg := schemas.MatchUserEvent{
				ProjectID: projectID,
				Name:      event.Name,
				Match:     *event.Match,
				Data:      event.Data,
			}
			err = srv.pubsub.Publish(ctx, schemas.UserEventsMatch(projectID), msg)

		default:
			msg := schemas.UserEvent{
				ProjectID: projectID,
				Name:      event.Name,
				Data:      event.Data,
			}
			switch {
			case ownScoped:
				msg.Identifiers = auth.BoundUserIdentifiers(ctx, nil)
			case event.Identifier != nil:
				msg.Identifiers = oapi.ToParams(*event.Identifier)
			}
			err = srv.pubsub.Publish(ctx, schemas.Subject(schemas.UserEventsProcess(projectID)), msg)
		}

		if err != nil {
			logger.Error("failed to publish event", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}
	}

	logger.Info("events processed successfully")
	w.WriteHeader(http.StatusAccepted)
}

func (srv *EventsController) PostOrganizationEventsClient(w http.ResponseWriter, r *http.Request, _ oapi.ProjectID) {
	projectID, err := srv.engine.AllowedProject(r.Context(), "events", rbac.Create)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	ctx := r.Context()

	// Organization events act across a whole organization, which an own-data end
	// user is never entitled to do.
	if err := auth.RequireCrossSubjectAccess(ctx); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var events oapi.PostOrganizationEventsRequest
	err = json.Decode(r.Body, &events)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Int("events", len(events)))
	logger.Info("posting organization events")

	names := make([]string, len(events))
	for i, event := range events {
		names[i] = event.Name
	}
	if err := srv.constraints.Enforce(ctx, rbac.ResourceEvents, names); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	for _, event := range events {
		if event.Match != nil && event.Identifier != nil {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("match and identifier are mutually exclusive")))
			return
		}
		if event.Match == nil && event.Identifier == nil {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("one of match or identifier is required")))
			return
		}

		var data map[string]any
		if event.Data != nil {
			data = *event.Data
		}

		switch {
		case event.Match != nil:
			msg := schemas.MatchOrganizationEvent{
				ProjectID: projectID,
				Name:      event.Name,
				Match:     *event.Match,
				Data:      data,
			}
			err = srv.pubsub.Publish(ctx, schemas.OrganizationEventsMatch(projectID), msg)

		case event.Identifier != nil:
			orgIdentifiers := oapi.ToParams(*event.Identifier)
			var orgID uuid.UUID
			orgID, err = srv.users.LookupOrganizationID(ctx, projectID, orgIdentifiers)
			if errors.Is(err, subjects.ErrOrgNotFound) {
				logger.Warn("organization not found, skipping event",
					zap.Int("org_identifiers", len(*event.Identifier)),
					zap.String("event_name", event.Name))
				continue
			}
			if err != nil {
				logger.Error("failed to lookup organization", zap.Error(err))
				oapi.WriteProblem(w, problem.ErrInternal())
				return
			}

			msg := schemas.OrganizationEvent{
				Name:                    event.Name,
				ProjectID:               projectID,
				OrganizationID:          orgID,
				OrganizationIdentifiers: orgIdentifiers,
				Data:                    data,
			}

			err = srv.pubsub.Publish(ctx, schemas.OrganizationEventsProcess(projectID), msg)
		}

		if err != nil {
			logger.Error("failed to publish organization event", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}
	}

	logger.Info("organization events processed successfully")
	w.WriteHeader(http.StatusAccepted)
}
