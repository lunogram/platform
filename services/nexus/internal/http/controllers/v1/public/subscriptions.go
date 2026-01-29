package v1

import (
	"database/sql"
	"embed"
	"errors"
	"html/template"
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/public/oapi"
	"github.com/lunogram/platform/services/nexus/internal/http/problem"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"go.uber.org/zap"
)

//go:embed templates/*.html
var templatesFS embed.FS

type SubscriptionsController struct {
	logger *zap.Logger
	db     *sqlx.DB
	store  *store.State
	tmpl   *template.Template
}

func NewSubscriptionsController(logger *zap.Logger, db *sqlx.DB) (*SubscriptionsController, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	return &SubscriptionsController{
		logger: logger,
		db:     db,
		store:  store.NewState(db),
		tmpl:   tmpl,
	}, nil
}

type PreferencesData struct {
	ProjectID          uuid.UUID
	UserID             uuid.UUID
	Subscriptions      []SubscriptionItem
	ShowSuccessMessage bool
}

type SubscriptionItem struct {
	SubscriptionID uuid.UUID
	Name           string
	Channel        string
	State          string
}

func (srv *SubscriptionsController) GetPreferencesPage(w http.ResponseWriter, r *http.Request, projectID, userID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("user_id", userID))
	logger.Info("getting preferences page")

	// Check if user exists
	_, err := srv.store.GetUser(ctx, projectID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("user not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}
	if err != nil {
		logger.Error("failed to get user", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	// Get all subscriptions for user-facing page
	subscriptions, err := srv.store.GetAllUserSubscriptions(ctx, projectID, userID)
	if err != nil {
		logger.Error("failed to get user subscriptions", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	// Convert to template format
	items := make([]SubscriptionItem, len(subscriptions))
	for i, sub := range subscriptions {
		items[i] = SubscriptionItem{
			SubscriptionID: sub.SubscriptionID,
			Name:           sub.Name,
			Channel:        sub.Channel,
			State:          sub.State,
		}
	}

	// Check for success message
	showSuccess := r.URL.Query().Get("u") == "1"

	data := PreferencesData{
		ProjectID:          projectID,
		UserID:             userID,
		Subscriptions:      items,
		ShowSuccessMessage: showSuccess,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = srv.tmpl.ExecuteTemplate(w, "preferences.html", data)
	if err != nil {
		logger.Error("failed to execute template", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (srv *SubscriptionsController) UpdatePreferences(w http.ResponseWriter, r *http.Request, projectID, userID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("user_id", userID))
	logger.Info("updating preferences")

	// Verify user exists
	_, err := srv.store.GetUser(ctx, projectID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("user not found")
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		logger.Error("failed to get user", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Parse form data
	err = r.ParseForm()
	if err != nil {
		logger.Error("failed to parse form", zap.Error(err))
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// Get selected subscription IDs
	selectedIDs := make(map[uuid.UUID]bool)
	for _, idStr := range r.Form["subscriptionIds"] {
		if id, err := uuid.Parse(idStr); err == nil {
			selectedIDs[id] = true
		}
	}

	// Get all user subscriptions
	subscriptions, err := srv.store.GetAllUserSubscriptions(ctx, projectID, userID)
	if err != nil {
		logger.Error("failed to get user subscriptions", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Update each subscription
	for _, sub := range subscriptions {
		state := "unsubscribed"
		if selectedIDs[sub.SubscriptionID] {
			state = "subscribed"
		}
		err = srv.store.ToggleSubscription(ctx, userID, sub.SubscriptionID, state)
		if err != nil {
			logger.Error("failed to toggle subscription", zap.Error(err))
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	logger.Info("preferences updated successfully")

	// Redirect back to preferences page with success indicator
	http.Redirect(w, r, "/preferences/"+projectID.String()+"/"+userID.String()+"?u=1", http.StatusSeeOther)
}

type UnsubscribeData struct {
}

func (srv *SubscriptionsController) EmailUnsubscribe(w http.ResponseWriter, r *http.Request, params oapi.EmailUnsubscribeParams) {
	ctx := r.Context()
	logger := srv.logger.With(zap.String("link", params.Link))
	logger.Info("processing email unsubscribe")

	// Parse the URL to extract query parameters
	// The link format from legacy code: ?u={user_id}&c={campaign_id}&s={reference_id}&r={redirect}
	parsedURL, err := url.Parse(params.Link)
	if err != nil {
		logger.Error("failed to parse link", zap.Error(err))
		http.Error(w, "Invalid link", http.StatusBadRequest)
		return
	}

	// Extract user_id and campaign_id from query parameters
	userIDStr := parsedURL.Query().Get("u")
	campaignIDStr := parsedURL.Query().Get("c")

	if userIDStr == "" || campaignIDStr == "" {
		logger.Error("missing required parameters in link")
		http.Error(w, "Invalid link format", http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		logger.Error("invalid user_id", zap.Error(err))
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	campaignID, err := uuid.Parse(campaignIDStr)
	if err != nil {
		logger.Error("invalid campaign_id", zap.Error(err))
		http.Error(w, "Invalid campaign ID", http.StatusBadRequest)
		return
	}

	// We need to get the user first to find their project_id
	// Since we don't know the project_id from the link, we need to query for the user
	// across projects. For now, we'll use a simpler approach: get campaign by ID only
	// and use its project_id
	
	// Query campaign across all projects to find the subscription_id
	query := `SELECT id, project_id, subscription_id FROM campaigns WHERE id = $1 AND deleted_at IS NULL`
	var campaignData struct {
		ID             uuid.UUID  `db:"id"`
		ProjectID      uuid.UUID  `db:"project_id"`
		SubscriptionID *uuid.UUID `db:"subscription_id"`
	}
	
	err = srv.db.GetContext(ctx, &campaignData, query, campaignID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("campaign not found")
		http.Error(w, "Campaign not found", http.StatusNotFound)
		return
	}
	if err != nil {
		logger.Error("failed to get campaign", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if campaignData.SubscriptionID == nil {
		logger.Error("campaign has no subscription_id")
		http.Error(w, "Invalid campaign", http.StatusBadRequest)
		return
	}

	// Get user to verify they exist
	_, err = srv.store.GetUser(ctx, campaignData.ProjectID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("user not found")
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	if err != nil {
		logger.Error("failed to get user", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Unsubscribe the user from this subscription type
	err = srv.store.ToggleSubscription(ctx, userID, *campaignData.SubscriptionID, "unsubscribed")
	if err != nil {
		logger.Error("failed to unsubscribe user", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	logger.Info("user successfully unsubscribed", 
		zap.Stringer("user_id", userID),
		zap.Stringer("campaign_id", campaignID),
		zap.Stringer("subscription_id", *campaignData.SubscriptionID))

	data := UnsubscribeData{}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = srv.tmpl.ExecuteTemplate(w, "unsubscribe.html", data)
	if err != nil {
		logger.Error("failed to execute template", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
