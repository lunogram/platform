package v1

import (
	"database/sql"
	"embed"
	"errors"
	"html/template"
	"net/http"

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

var translations = map[string]map[string]string{
	"en": {
		"title":               "Communication Preferences",
		"description":         "Choose which methods of communication you would like to continue to receive:",
		"success_message":     "Your preferences have been updated!",
		"no_subscriptions":    "You are not subscribed to any notifications.",
		"save":                "Save Preferences",
		"unsubscribed":        "You have been unsubscribed!",
		"unsubscribed_message": "You will no longer receive these communications.",
	},
	"es": {
		"title":               "Preferencias de Comunicación",
		"description":         "Elige qué métodos de comunicación deseas seguir recibiendo:",
		"success_message":     "¡Tus preferencias han sido actualizadas!",
		"no_subscriptions":    "No estás suscrito a ninguna notificación.",
		"save":                "Guardar Preferencias",
		"unsubscribed":        "¡Te has dado de baja!",
		"unsubscribed_message": "Ya no recibirás estas comunicaciones.",
	},
	"fr": {
		"title":               "Préférences de communication",
		"description":         "Choisissez les méthodes de communication que vous souhaitez continuer à recevoir :",
		"success_message":     "Vos préférences ont été mises à jour !",
		"no_subscriptions":    "Vous n'êtes abonné à aucune notification.",
		"save":                "Enregistrer les Préférences",
		"unsubscribed":        "Vous avez été désabonné !",
		"unsubscribed_message": "Vous ne recevrez plus ces communications.",
	},
}

func getTranslations(locale string) map[string]string {
	if locale == "" {
		locale = "en"
	}
	// Extract base locale (e.g., "en" from "en-US")
	if len(locale) > 2 {
		locale = locale[:2]
	}
	if trans, ok := translations[locale]; ok {
		return trans
	}
	return translations["en"]
}

type PreferencesData struct {
	Title               string
	Description         string
	SuccessMessage      string
	NoSubscriptionsText string
	SaveButtonText      string
	Locale              string
	ProjectID           uuid.UUID
	UserID              uuid.UUID
	Subscriptions       []SubscriptionItem
	ShowSuccessMessage  bool
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

	// Get user to determine locale
	user, err := srv.store.GetUser(ctx, projectID, userID)
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

	// Get all subscriptions without pagination limit for user-facing page
	pagination := store.Pagination{Limit: 1000, Offset: 0}
	subscriptions, _, err := srv.store.GetUserSubscriptions(ctx, projectID, userID, pagination)
	if err != nil {
		logger.Error("failed to get user subscriptions", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	// Get locale from user or default to "en"
	locale := "en"
	if user.Locale != nil && *user.Locale != "" {
		locale = *user.Locale
	}

	trans := getTranslations(locale)
	
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
		Title:               trans["title"],
		Description:         trans["description"],
		SuccessMessage:      trans["success_message"],
		NoSubscriptionsText: trans["no_subscriptions"],
		SaveButtonText:      trans["save"],
		Locale:              locale,
		ProjectID:           projectID,
		UserID:              userID,
		Subscriptions:       items,
		ShowSuccessMessage:  showSuccess,
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

	// Parse form data
	err := r.ParseForm()
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

	// Get all user subscriptions without pagination limit
	pagination := store.Pagination{Limit: 1000, Offset: 0}
	subscriptions, _, err := srv.store.GetUserSubscriptions(ctx, projectID, userID, pagination)
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
	Title   string
	Message string
	Locale  string
}

func (srv *SubscriptionsController) EmailUnsubscribe(w http.ResponseWriter, r *http.Request, params oapi.EmailUnsubscribeParams) {
	logger := srv.logger.With(zap.String("link", params.Link))
	logger.Info("processing email unsubscribe")

	// TODO: Implement full unsubscribe logic
	// The link format from the legacy TypeScript code suggests it contains encoded user and campaign info
	// This needs to be decoded to extract:
	// - user_id: to identify which user is unsubscribing
	// - campaign_id or subscription_id: to identify what they're unsubscribing from
	// - signature: to verify the link hasn't been tampered with
	//
	// For now, we display a confirmation message. The actual unsubscribe logic needs to:
	// 1. Decode and verify the link
	// 2. Extract user_id and subscription_id
	// 3. Call srv.store.ToggleSubscription(ctx, userID, subscriptionID, "unsubscribed")
	// 4. Get user locale for proper message display

	// Default to English for now
	locale := "en"
	trans := getTranslations(locale)

	data := UnsubscribeData{
		Title:   trans["unsubscribed"],
		Message: trans["unsubscribed_message"],
		Locale:  locale,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := srv.tmpl.ExecuteTemplate(w, "unsubscribe.html", data)
	if err != nil {
		logger.Error("failed to execute template", zap.Error(err))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
