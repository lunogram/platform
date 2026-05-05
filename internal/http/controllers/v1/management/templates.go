package v1

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/providers/channels"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/render"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	moduleProviders "github.com/lunogram/platform/pkg/modules/providers"

	"go.uber.org/zap"
)

// templateDataEnvelope partially unmarshals email template data so that the
// code block can be inspected and mutated in a type-safe way, while all other
// fields (e.g. blocks, editorMode) are preserved via the embedded RawMessage.
type templateDataEnvelope struct {
	Code channels.EmailCodeData `json:"code,omitempty"`

	// Remaining holds every top-level field *except* "code". We use it to
	// reconstruct the full JSON after mutating Code.
	Remaining map[string]json.RawMessage `json:"-"`
}

func (t *templateDataEnvelope) UnmarshalJSON(data []byte) error {
	// Unmarshal all top-level keys into the generic map.
	if err := json.Unmarshal(data, &t.Remaining); err != nil {
		return err
	}
	// Unmarshal the typed Code field from the "code" key, if present.
	if raw, ok := t.Remaining["code"]; ok {
		if err := json.Unmarshal(raw, &t.Code); err != nil {
			return err
		}
		delete(t.Remaining, "code")
	}
	return nil
}

func (t templateDataEnvelope) MarshalJSON() ([]byte, error) {
	// Start from a copy of the remaining fields so we don't mutate the original.
	merged := make(map[string]json.RawMessage, len(t.Remaining)+1)
	for k, v := range t.Remaining {
		merged[k] = v
	}
	codeBytes, err := json.Marshal(t.Code)
	if err != nil {
		return nil, err
	}
	merged["code"] = codeBytes
	return json.Marshal(merged)
}

func NewTemplatesController(logger *zap.Logger, db *sqlx.DB, subjectsDB *sqlx.DB, renderer *pubsub.EmailRenderer, registry *providers.Registry, engine *rbac.Engine, linkKey []byte, trackingURL string) *TemplatesController {
	return &TemplatesController{
		logger:      logger,
		db:          db,
		store:       management.NewState(db),
		subjects:    subjects.NewState(subjectsDB, logger),
		renderer:    renderer,
		registry:    registry,
		engine:      engine,
		linkKey:     linkKey,
		trackingURL: trackingURL,
	}
}

type TemplatesController struct {
	logger      *zap.Logger
	db          *sqlx.DB
	store       *management.State
	renderer    *pubsub.EmailRenderer
	registry    *providers.Registry
	engine      *rbac.Engine
	linkKey     []byte
	trackingURL string
	subjects    *subjects.State
}

func (srv *TemplatesController) GetTemplate(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, campaignID uuid.UUID, templateID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("templates", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID), zap.Stringer("template_id", templateID))
	logger.Info("getting template")

	template, err := srv.store.TemplatesStore.GetTemplate(ctx, projectID, templateID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("template not found", zap.Stringer("template_id", templateID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("template not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get template", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	json.Write(w, http.StatusOK, template.OAPI())
}

func (srv *TemplatesController) CreateTemplate(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, campaignID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("templates", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}
	body := oapi.CreateTemplate{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	campaign, err := srv.store.CampaignsStore.GetCampaign(ctx, projectID, campaignID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("campaign not found", zap.Stringer("campaign_id", campaignID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("campaign not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID), zap.String("type", campaign.Channel))
	logger.Info("creating template")

	var senderIdentityID *uuid.UUID
	if body.SenderIdentityId != nil {
		id := uuid.UUID(*body.SenderIdentityId)
		senderIdentityID = &id
	}

	templateID, err := srv.store.TemplatesStore.CreateTemplate(ctx, projectID, campaignID, campaign.Channel, body.Locale, senderIdentityID)
	if err != nil {
		logger.Error("failed to create template", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if body.Data != nil {
		update := management.TemplateUpdate{
			Data: body.Data,
		}

		err = srv.store.TemplatesStore.UpdateTemplate(ctx, projectID, templateID, update)
		if err != nil {
			logger.Error("failed to update template data", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
	}

	logger.Info("template created")
	template, err := srv.store.TemplatesStore.GetTemplate(ctx, projectID, templateID)
	if err != nil {
		logger.Error("failed to fetch created template", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	json.Write(w, http.StatusCreated, template.OAPI())
}

func (srv *TemplatesController) DeleteTemplate(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, campaignID uuid.UUID, templateID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("templates", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID), zap.Stringer("template_id", templateID))
	logger.Info("deleting template")

	_, err = srv.store.TemplatesStore.GetTemplate(ctx, projectID, templateID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("template not found", zap.Stringer("template_id", templateID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("template not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get template", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	err = srv.store.TemplatesStore.DeleteTemplate(ctx, projectID, templateID)
	if err != nil {
		logger.Error("failed to delete template", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("template deleted")
	w.WriteHeader(http.StatusNoContent)
}

func (srv *TemplatesController) UpdateTemplate(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, campaignID uuid.UUID, templateID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("templates", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID), zap.Stringer("template_id", templateID))
	logger.Info("updating template")

	var body oapi.UpdateTemplate
	if err := json.Decode(r.Body, &body); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.store.TemplatesStore.GetTemplate(ctx, projectID, templateID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("template not found", zap.Stringer("template_id", templateID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("template not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get template", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// If the template data contains React Email source code, compile it via
	// the Deno renderer service and store the compiled JS alongside the source.
	// Extra fields stored by the frontend (e.g. blocks, editorMode) are
	// preserved through the marshal/unmarshal round-trip via templateDataEnvelope.
	if body.Data != nil {
		var envelope templateDataEnvelope
		if err := json.Unmarshal(*body.Data, &envelope); err != nil {
			logger.Error("failed to unmarshal template data", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to unmarshal template data")))
			return
		}

		if envelope.Code.Source != "" {
			envelope.Code.Bundle, envelope.Code.BundleHash, err = srv.renderer.Compile(ctx, projectID, envelope.Code.Source)
			if err != nil {
				logger.Error("failed to compile template", zap.Error(err))
				oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to compile email template")))
				return
			}

			updated, _ := json.Marshal(envelope) //nolint:errcheck
			raw := json.RawMessage(updated)
			body.Data = &raw
		}
	}

	updated := management.TemplateUpdate{
		Data: body.Data,
	}

	// Handle sender_identity_id: the field is nullable, so we need to distinguish
	// between "not provided" (nil pointer) and "explicitly set to null" (provided but null).
	// Since oapi-codegen generates *uuid for nullable UUID, nil means not provided
	// and we leave it unchanged. A non-nil value sets the new identity.
	// To clear, the frontend sends the field with a null value — but since Go
	// deserialises null UUID as uuid.Nil, we treat uuid.Nil as "clear".
	if body.SenderIdentityId != nil {
		id := uuid.UUID(*body.SenderIdentityId)
		if id == uuid.Nil {
			updated.ClearSenderIdentityID = true
		} else {
			updated.SenderIdentityID = &id
		}
	}

	err = srv.store.TemplatesStore.UpdateTemplate(ctx, projectID, templateID, updated)
	if err != nil {
		logger.Error("failed to update template", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	template, err := srv.store.TemplatesStore.GetTemplate(ctx, projectID, templateID)
	if err != nil {
		logger.Error("failed to fetch updated template", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("template updated")
	json.Write(w, http.StatusOK, template.OAPI())
}

func (srv *TemplatesController) SendTest(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, campaignID uuid.UUID, templateID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("templates", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("campaign_id", campaignID), zap.Stringer("template_id", templateID))
	logger.Info("sending test")

	var body oapi.SendTest
	if err := json.Decode(r.Body, &body); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	// Get the template.
	template, err := srv.store.TemplatesStore.GetTemplate(ctx, projectID, templateID)
	if errors.Is(err, sql.ErrNoRows) {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("template not found")))
		return
	}
	if err != nil {
		logger.Error("failed to get template", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Get the campaign to find the channel.
	campaign, err := srv.store.CampaignsStore.GetCampaign(ctx, projectID, campaignID)
	if errors.Is(err, sql.ErrNoRows) {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("campaign not found")))
		return
	}
	if err != nil {
		logger.Error("failed to get campaign", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	props := make(map[string]any)
	if body.Props != nil {
		props = *body.Props
	}

	templateData := template.Data

	if campaign.Channel == "email" {
		templateData, err = channels.ComposeEmailTemplateData(ctx, srv.renderer, projectID, templateData, props)
		if err != nil {
			logger.Error("failed to compose template data", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to compose email template")))
			return
		}
	}

	templateData, err = render.RenderJSON(templateData, props)
	if err != nil {
		logger.Error("failed to render template data", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to render template data")))
		return
	}

	// Push channel: resolve provider from project push providers and compose
	// a push request. For backward compatibility, "to" can still be a raw
	// token. Prefer passing a registered device_id so we can resolve the full
	// push config (including Web Push endpoint/keys).
	if campaign.Channel == "push" {
		if body.To == "" && (body.Push == nil || body.Push.DeviceId == "") {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("to is required")))
			return
		}

		var pushData channels.PushTemplateData
		if err := json.Unmarshal(templateData, &pushData); err != nil {
			logger.Error("failed to unmarshal push template data", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to parse push template data")))
			return
		}

		custom := pushData.Data
		if custom == nil {
			custom = make(map[string]any)
		}

		payload := moduleProviders.PushPayload{
			Title: pushData.Title,
			Body:  pushData.Body,
			Data:  custom,
		}

		providerPlatform := ""

		targetDeviceID := body.To
		if body.Push != nil && body.Push.DeviceId != "" {
			targetDeviceID = body.Push.DeviceId
		}

		device, err := srv.subjects.GetDeviceByProjectAndDeviceID(ctx, projectID, targetDeviceID)
		switch {
		case err == nil:
			if device.Config == nil {
				oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("selected device has no push configuration")))
				return
			}

			switch device.Config.Type {
			case subjects.PushConfigTypeFCM:
				if device.Config.Token == "" {
					oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("selected device has no push token")))
					return
				}
				payload.Tokens = []string{device.Config.Token}
				providerPlatform = management.PlatformAndroid
			case subjects.PushConfigTypeAPNs:
				if device.Config.Token == "" {
					oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("selected device has no push token")))
					return
				}
				payload.APNsTokens = []string{device.Config.Token}
				providerPlatform = management.PlatformIOS
			case subjects.PushConfigTypeWebPush:
				if device.Config.Endpoint == "" || device.Config.Keys == nil || device.Config.Keys.Auth == "" || device.Config.Keys.P256dh == "" {
					oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("selected web push device has incomplete subscription data")))
					return
				}

				target := moduleProviders.WebPushTarget{Endpoint: device.Config.Endpoint}
				if device.Config.ExpirationTime != nil {
					exp := device.Config.ExpirationTime.Unix()
					target.ExpirationTime = &exp
				}
				target.Keys.Auth = device.Config.Keys.Auth
				target.Keys.P256dh = device.Config.Keys.P256dh
				payload.WebPushTargets = []moduleProviders.WebPushTarget{target}
				providerPlatform = management.PlatformWeb
			default:
				oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("unsupported device push configuration type")))
				return
			}
		case errors.Is(err, sql.ErrNoRows) && (body.Push == nil || body.Push.DeviceId == ""):
			// Backward compatibility: treat "to" as a direct push token.
			payload.Tokens = []string{body.To}
		case errors.Is(err, sql.ErrNoRows):
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("selected device not found")))
			return
		case err != nil:
			logger.Error("failed to resolve device for push test", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}

		var provider *management.Provider
		if providerPlatform != "" {
			pp, err := srv.store.ProjectProvidersStore.GetProjectProvider(ctx, projectID, providerPlatform)
			if errors.Is(err, sql.ErrNoRows) {
				oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("no push provider configured for selected device platform")))
				return
			}
			if err != nil {
				logger.Error("failed to get project push provider", zap.Error(err), zap.String("platform", providerPlatform))
				oapi.WriteProblem(w, err)
				return
			}

			provider, err = srv.store.ProvidersStore.GetProvider(ctx, pp.ProviderID)
			if err != nil {
				logger.Error("failed to get push provider", zap.Error(err))
				oapi.WriteProblem(w, err)
				return
			}
		} else {
			pushProviders, err := srv.store.ProjectProvidersStore.ListProjectProviders(ctx, projectID)
			if err != nil {
				logger.Error("failed to list push providers", zap.Error(err))
				oapi.WriteProblem(w, err)
				return
			}
			if len(pushProviders) == 0 {
				oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("project has no push providers configured")))
				return
			}

			provider, err = srv.store.ProvidersStore.GetProvider(ctx, pushProviders[0].ProviderID)
			if err != nil {
				logger.Error("failed to get push provider", zap.Error(err))
				oapi.WriteProblem(w, err)
				return
			}
		}

		module, ok := srv.registry.Get(provider.Module)
		if !ok {
			logger.Error("provider module not found", zap.String("module", provider.Module))
			oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("provider module not found")))
			return
		}

		var config map[string]any
		if err := json.Unmarshal(provider.Data, &config); err != nil {
			logger.Error("failed to parse provider config", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to parse provider config")))
			return
		}

		request, err := moduleProviders.NewPushRequest(config, payload)
		if err != nil {
			logger.Error("failed to compose push request", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to compose push request")))
			return
		}

		sendResp, err := module.Send(ctx, request)
		if err != nil {
			logger.Error("failed to send push test", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to send test: "+err.Error())))
			return
		}

		logger.Info("push test sent",
			zap.String("to", string(body.To)),
			zap.String("message_id", sendResp.ID),
			zap.String("status", sendResp.Status),
		)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Email/SMS: resolve provider from template sender identity.
	if template.SenderIdentityID == nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("template has no sender identity configured")))
		return
	}

	senderIdentity, err := srv.store.SenderIdentitiesStore.GetSenderIdentity(ctx, projectID, *template.SenderIdentityID)
	if err != nil {
		logger.Error("failed to get sender identity", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to resolve sender identity")))
		return
	}

	provider, err := srv.store.ProvidersStore.GetProvider(ctx, senderIdentity.ProviderID)
	if err != nil {
		logger.Error("failed to get provider", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	module, ok := srv.registry.Get(provider.Module)
	if !ok {
		logger.Error("provider module not found", zap.String("module", provider.Module))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("provider module not found")))
		return
	}

	var config map[string]any
	err = json.Unmarshal(provider.Data, &config)
	if err != nil {
		logger.Error("failed to parse provider config", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to parse provider config")))
		return
	}

	// Resolve template sender identity if set.
	templateSender := senderIdentity

	// Resolve provider default_from.
	providerDefaultSender, err := channels.ResolveProviderDefaultFrom(ctx, srv.store.SenderIdentitiesStore, projectID, config)
	if err != nil {
		logger.Error("failed to resolve provider default from", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to resolve provider default from")))
		return
	}

	var wrapper *channels.LinkWrapConfig
	if len(srv.linkKey) > 0 && srv.trackingURL != "" && provider.LinkWrap {
		wrapper = &channels.LinkWrapConfig{
			Key:         srv.linkKey,
			TrackingURL: srv.trackingURL,
			ProjectID:   projectID,
			CampaignID:  campaignID,
		}

		// TODO: we might want to create a type for props
		props, ok := props["user"].(map[string]any)
		if ok {
			id, ok := props["id"].(string)
			if ok {
				wrapper.UserID, _ = uuid.Parse(id)
			}
		}
	}

	request, err := channels.ComposePayload(ctx, logger, campaign.Channel, templateSender, providerDefaultSender, config, templateData, string(body.To), wrapper)
	if err != nil {
		logger.Warn("failed to compose payload", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe(err.Error())))
		return
	}

	sendResp, err := module.Send(ctx, request)
	if err != nil {
		logger.Error("failed to send test", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal(problem.Describe("failed to send test: "+err.Error())))
		return
	}

	logger.Info("test sent",
		zap.String("to", string(body.To)),
		zap.String("message_id", sendResp.ID),
		zap.String("status", sendResp.Status),
	)
	w.WriteHeader(http.StatusNoContent)
}
