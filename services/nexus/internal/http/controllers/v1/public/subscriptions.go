package v1

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/public/templates"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"go.uber.org/zap"
)

func NewSubscriptionsController(logger *zap.Logger, db *sqlx.DB) *SubscriptionsController {
	return &SubscriptionsController{
		logger: logger,
		db:     db,
		store:  store.NewStores(db),
	}
}

type SubscriptionsController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *store.Stores
}

// UnsubscribeEmail handles the email unsubscribe link
// It expects query parameters: user_id, campaign_id, and signature
func (srv *SubscriptionsController) UnsubscribeEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	userIDStr := r.URL.Query().Get("user_id")
	campaignIDStr := r.URL.Query().Get("campaign_id")

	if userIDStr == "" || campaignIDStr == "" {
		srv.logger.Error("missing required parameters")
		http.Error(w, "Invalid unsubscribe link", http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		srv.logger.Error("invalid user ID", zap.Error(err))
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		srv.logger.Error("invalid campaign ID", zap.Error(err))
		http.Error(w, "Invalid campaign ID", http.StatusBadRequest)
		return
	}

	logger := srv.logger.With(
		zap.String("user_id", userID.String()),
		zap.String("campaign_id", campaignID.String()),
	)

	// Get the campaign to find its subscription_id
	campaign, err := srv.store.GetCampaignByID(ctx, campaignID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Error("campaign not found")
			http.Error(w, "Campaign not found", http.StatusNotFound)
			return
		}
		logger.Error("failed to get campaign", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if campaign.SubscriptionID == nil {
		logger.Error("campaign has no subscription")
		http.Error(w, "Campaign has no subscription", http.StatusBadRequest)
		return
	}

	// Get user to determine locale
	user, err := srv.store.GetUser(ctx, campaign.ProjectID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Error("user not found")
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		logger.Error("failed to get user", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Unsubscribe the user
	err = srv.store.ToggleSubscription(ctx, userID, *campaign.SubscriptionID, "unsubscribed")
	if err != nil {
		logger.Error("failed to unsubscribe user", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	logger.Info("user unsubscribed from campaign")

	// Render the unsubscribe confirmation page
	locale := "en"
	if user.Locale != nil {
		locale = *user.Locale
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	err = templates.RenderUnsubscribe(w, locale)
	if err != nil {
		logger.Error("failed to render template", zap.Error(err))
	}
}

// GetPreferences displays the subscription preferences page
func (srv *SubscriptionsController) GetPreferences(w http.ResponseWriter, r *http.Request, userIDStr string) {
	ctx := r.Context()

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		srv.logger.Error("invalid user ID", zap.Error(err))
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	logger := srv.logger.With(zap.String("user_id", userID.String()))

	// Get user to determine project and locale
	// We need to get the user's project somehow - for now we'll get it from the first subscription
	// This is a limitation since we don't have project_id in the URL
	// In a real scenario, you might want to add project_id to the URL or use a different approach

	// For now, let's get all subscriptions across projects (not ideal but works for single project setups)
	// We'll need to modify this to get the user first with their project_id

	// Actually, we need the project_id. Let's get the user by ID across all projects
	// This requires a different store method. For now, let's require project_id in query params

	projectIDStr := r.URL.Query().Get("project_id")
	if projectIDStr == "" {
		srv.logger.Error("missing project_id parameter")
		http.Error(w, "Missing project_id parameter", http.StatusBadRequest)
		return
	}

	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		srv.logger.Error("invalid project ID", zap.Error(err))
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	logger = logger.With(zap.String("project_id", projectID.String()))

	user, err := srv.store.GetUser(ctx, projectID, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			logger.Error("user not found")
			http.Error(w, "User not found", http.StatusNotFound)
			return
		}
		logger.Error("failed to get user", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Get user subscriptions
	pagination := store.Pagination{Limit: 100, Offset: 0}
	subscriptions, _, err := srv.store.GetUserSubscriptions(ctx, projectID, userID, pagination)
	if err != nil {
		logger.Error("failed to get user subscriptions", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	logger.Info("rendering preferences page", zap.Int("subscriptions", len(subscriptions)))

	locale := "en"
	if user.Locale != nil {
		locale = *user.Locale
	}

	showUpdated := r.URL.Query().Get("u") == "1"

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	err = templates.RenderPreferences(w, templates.PreferencesData{
		UserID:             userID.String(),
		Locale:             locale,
		Subscriptions:      subscriptions.OAPI(),
		ShowUpdatedMessage: showUpdated,
	})
	if err != nil {
		logger.Error("failed to render template", zap.Error(err))
	}
}

// UpdatePreferences handles the form submission to update subscription preferences
func (srv *SubscriptionsController) UpdatePreferences(w http.ResponseWriter, r *http.Request, userIDStr string) {
	ctx := r.Context()

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		srv.logger.Error("invalid user ID", zap.Error(err))
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	logger := srv.logger.With(zap.String("user_id", userID.String()))

	projectIDStr := r.URL.Query().Get("project_id")
	if projectIDStr == "" {
		srv.logger.Error("missing project_id parameter")
		http.Error(w, "Missing project_id parameter", http.StatusBadRequest)
		return
	}

	projectID, err := uuid.Parse(projectIDStr)
	if err != nil {
		srv.logger.Error("invalid project ID", zap.Error(err))
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	logger = logger.With(zap.String("project_id", projectID.String()))

	// Parse form data
	err = r.ParseForm()
	if err != nil {
		logger.Error("failed to parse form", zap.Error(err))
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	selectedIDs := r.Form["subscription_ids"]
	selectedMap := make(map[string]bool)
	for _, id := range selectedIDs {
		selectedMap[id] = true
	}

	logger.Info("updating preferences", zap.Strings("selected_ids", selectedIDs))

	// Get all subscriptions for this user
	pagination := store.Pagination{Limit: 100, Offset: 0}
	subscriptions, _, err := srv.store.GetUserSubscriptions(ctx, projectID, userID, pagination)
	if err != nil {
		logger.Error("failed to get user subscriptions", zap.Error(err))
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Update each subscription based on whether it was selected
	for _, sub := range subscriptions {
		newState := "unsubscribed"
		if selectedMap[sub.SubscriptionID.String()] {
			newState = "subscribed"
		}

		err = srv.store.ToggleSubscription(ctx, userID, sub.SubscriptionID, newState)
		if err != nil {
			logger.Error("failed to toggle subscription",
				zap.String("subscription_id", sub.SubscriptionID.String()),
				zap.Error(err))
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	logger.Info("preferences updated successfully")

	// Redirect back to preferences page with success message
	redirectURL := "/preferences/" + userIDStr + "?project_id=" + projectID.String() + "&u=1"
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// ServeStaticFiles serves the static CSS files
func (srv *SubscriptionsController) ServeStaticFiles(w http.ResponseWriter, r *http.Request) {
	// Remove "/static/" prefix from path
	path := strings.TrimPrefix(r.URL.Path, "/static/")
	if path == "" || path == "/" {
		http.NotFound(w, r)
		return
	}

	// Read file from embedded FS
	content, err := templates.StaticFiles().ReadFile("static/" + path)
	if err != nil {
		srv.logger.Error("failed to read static file", zap.String("path", path), zap.Error(err))
		http.NotFound(w, r)
		return
	}

	// Set appropriate content type
	if strings.HasSuffix(path, ".css") {
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	}

	w.WriteHeader(http.StatusOK)
	w.Write(content)
}
