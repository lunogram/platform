package v1

import (
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/providers"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/management"
	wasmProviders "github.com/lunogram/platform/internal/wasm/providers"
	providerpkg "github.com/lunogram/platform/pkg/modules/providers"
	"go.uber.org/zap"
)

// ProviderWebhookHandler handles inbound provider webhook callbacks.
// Provider webhooks (Resend, Twilio, SendGrid, etc.) POST delivery status
// updates to this endpoint. The handler delegates parsing and signature
// verification to the provider's WASM module, then publishes the resulting
// events to the providers.webhooks.{project_id} NATS subject for downstream
// processing.
func ProviderWebhookHandler(logger *zap.Logger, mgmt *management.State, registry *providers.Registry, pub pubsub.Publisher, maxBodySize int64, baseURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		projectID, err := uuid.Parse(chi.URLParam(r, "projectID"))
		if err != nil {
			http.Error(w, "invalid project ID", http.StatusBadRequest)
			return
		}

		providerID, err := uuid.Parse(chi.URLParam(r, "providerID"))
		if err != nil {
			http.Error(w, "invalid provider ID", http.StatusBadRequest)
			return
		}

		provider, err := mgmt.ProvidersStore.GetProviderByProject(ctx, projectID, providerID)
		if err != nil {
			logger.Warn("provider not found for webhook", zap.Stringer("project_id", projectID), zap.Stringer("provider_id", providerID), zap.Error(err))
			http.Error(w, "provider not found", http.StatusNotFound)
			return
		}

		module, exists := registry.Get(provider.Module)
		if !exists {
			logger.Error("WASM module not found for webhook",
				zap.String("module", provider.Module),
			)
			http.Error(w, "provider module not found", http.StatusNotFound)
			return
		}

		// Read the raw body, capped at the configured maximum size.
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
		if err != nil {
			logger.Error("failed to read webhook body", zap.Error(err))
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}

		// Collect headers (lowercase keys)
		headers := make(map[string]string, len(r.Header))
		for key, values := range r.Header {
			headers[strings.ToLower(key)] = values[0]
		}

		// Build the full request URL for signature verification.
		// Some providers (e.g. Twilio) include the callback URL in their HMAC.
		// We use the configured public URL rather than r.Host so that the
		// reconstructed URL matches the external address even when the
		// service sits behind a reverse proxy.
		fullURL := baseURL + r.RequestURI
		req := providerpkg.WebhookRequest{
			Config:  provider.Data,
			Headers: headers,
			Body:    body,
			URL:     fullURL,
		}

		res, err := module.Webhook(ctx, req)
		if err != nil {
			var providerErr *wasmProviders.ProviderError
			if errors.As(err, &providerErr) && providerErr.IsPermanent() {
				logger.Warn("webhook rejected by provider (invalid signature)",
					zap.String("module", provider.Module),
					zap.Error(err),
				)
				http.Error(w, "webhook rejected", http.StatusUnauthorized)
				return
			}

			logger.Error("webhook processing failed", zap.Error(err))
			http.Error(w, "webhook processing failed", http.StatusInternalServerError)
			return
		}

		// Publish each event to NATS on the provider webhooks subject.
		// Events are validated against the canonical set of provider event names
		// so that WASM modules cannot inject arbitrary event names into the pipeline.
		for _, event := range res.Events {
			webhookEvent := schemas.ProviderWebhookEvent{
				ProjectID:  projectID,
				ProviderID: providerID,
				Module:     provider.Module,
				Channel:    provider.Channel,
				EventName:  event.EventName.String(),
				MessageID:  event.MessageID,
				Timestamp:  event.Timestamp,
				Data:       event.Data,
			}

			err = pub.Publish(ctx, schemas.ProvidersWebhook(projectID), webhookEvent)
			if err != nil {
				logger.Error("failed to publish webhook event",
					zap.String("event_name", event.EventName.String()),
					zap.Error(err),
				)
			}
		}

		logger.Info("webhook processed",
			zap.Stringer("project_id", projectID),
			zap.String("module", provider.Module),
			zap.Int("events", len(res.Events)),
		)

		w.WriteHeader(http.StatusOK)
	}
}
