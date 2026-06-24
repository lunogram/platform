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
	"github.com/lunogram/platform/internal/http/controllers/v1/client/oapi"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

//go:embed templates/*.html
var templatesFS embed.FS

type SubscriptionsController struct {
	*ClientController
	db   *sqlx.DB
	mgmt *management.State
	tmpl *template.Template
}

func NewSubscriptionsController(client *ClientController, db *sqlx.DB, mgmt *management.State) (*SubscriptionsController, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	return &SubscriptionsController{
		ClientController: client,
		db:               db,
		mgmt:             mgmt,
		tmpl:             tmpl,
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
	_, err := srv.users.GetUser(ctx, projectID, userID)
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
	subscriptions, err := srv.mgmt.GetAllUserSubscriptions(ctx, projectID, userID)
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

	_, err := srv.users.GetUser(ctx, projectID, userID)
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

	err = r.ParseForm()
	if err != nil {
		logger.Error("failed to parse form", zap.Error(err))
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	selected := make(map[uuid.UUID]bool)
	for _, idStr := range r.Form["subscriptionIds"] {
		if id, err := uuid.Parse(idStr); err == nil {
			selected[id] = true
		}
	}

	subscriptions, err := srv.mgmt.GetAllUserSubscriptions(ctx, projectID, userID)
	if err != nil {
		logger.Error("failed to get user subscriptions", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	tx, err := srv.db.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback() //nolint:errcheck

	mgmtTx := management.NewState(tx)
	for _, sub := range subscriptions {
		subscribed := selected[sub.SubscriptionID]
		err = mgmtTx.SetSubscriptionState(ctx, userID, sub.SubscriptionID, subscribed)
		if err != nil {
			logger.Error("failed to update subscription", zap.Error(err))
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		logger.Error("failed to commit transaction", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	logger.Info("preferences updated successfully")
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
	link, err := url.Parse(params.Link)
	if err != nil {
		logger.Error("failed to parse link", zap.Error(err))
		http.Error(w, "Invalid link", http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(link.Query().Get("u"))
	if err != nil {
		logger.Error("invalid user_id", zap.Error(err))
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	campaignID, err := uuid.Parse(link.Query().Get("c"))
	if err != nil {
		logger.Error("invalid campaign_id", zap.Error(err))
		http.Error(w, "Invalid campaign ID", http.StatusBadRequest)
		return
	}

	campaign, err := srv.mgmt.GetCampaignByID(ctx, campaignID)
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

	if campaign.SubscriptionID == nil {
		logger.Error("campaign has no subscription_id")
		http.Error(w, "Invalid campaign", http.StatusBadRequest)
		return
	}

	_, err = srv.users.GetUser(ctx, campaign.ProjectID, userID)
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

	err = srv.mgmt.Unsubscribe(ctx, userID, *campaign.SubscriptionID)
	if err != nil {
		logger.Error("failed to unsubscribe user", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	logger.Info("user successfully unsubscribed",
		zap.Stringer("user_id", userID),
		zap.Stringer("campaign_id", campaignID),
		zap.Stringer("subscription_id", *campaign.SubscriptionID))

	data := UnsubscribeData{}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = srv.tmpl.ExecuteTemplate(w, "unsubscribe.html", data)
	if err != nil {
		logger.Error("failed to execute template", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
