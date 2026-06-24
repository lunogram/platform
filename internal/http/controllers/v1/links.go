package v1

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/providers/linkwrap"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"go.uber.org/zap"
)

// LinkRedirectHandler handles click tracking redirects.
// It decrypts the token, publishes a click event, and redirects to the original URL.
func LinkRedirectHandler(logger *zap.Logger, key []byte, pub pubsub.Publisher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := chi.URLParam(r, "token")
		if token == "" {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		payload, err := linkwrap.Decrypt(key, token)
		if err != nil {
			logger.Debug("invalid link token", zap.Error(err))
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		// Validate the decrypted URL before redirecting
		if !linkwrap.IsSafeRedirectURL(payload.URL) {
			logger.Warn("unsafe redirect URL in link token", zap.String("url", payload.URL), zap.Stringer("project_id", payload.ProjectID))
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		// Only publish click events for real users; test sends use uuid.Nil
		// and have no user to attribute the event to.
		if payload.UserID != uuid.Nil {
			go func() {
				ctx := context.Background()

				event := schemas.UserEvent{
					Name:      "link.clicked",
					ProjectID: payload.ProjectID,
					UserID:    payload.UserID,
					Data: map[string]any{
						"campaign_id":  payload.CampaignID.String(),
						"original_url": payload.URL,
					},
				}

				err = pub.Publish(ctx, schemas.UserEventsProcess(payload.ProjectID), event)
				if err != nil {
					logger.Error("failed to publish link click event", zap.Stringer("project_id", payload.ProjectID), zap.Stringer("campaign_id", payload.CampaignID), zap.Stringer("user_id", payload.UserID), zap.Error(err))
				}
			}()
		}

		// 302 redirect to the original URL
		http.Redirect(w, r, payload.URL, http.StatusFound)
	}
}
