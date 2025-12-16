package templates

import (
	"embed"
	"html/template"
	"io"

	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
)

//go:embed *.html static/*.css
var templateFiles embed.FS

var (
	unsubscribeTemplate *template.Template
	preferencesTemplate *template.Template
)

func init() {
	var err error
	unsubscribeTemplate, err = template.ParseFS(templateFiles, "unsubscribe.html")
	if err != nil {
		panic(err)
	}

	preferencesTemplate, err = template.ParseFS(templateFiles, "preferences.html")
	if err != nil {
		panic(err)
	}
}

// Strings contains localized strings for the templates
type Strings struct {
	Title               string
	Description         string
	SuccessMessage      string
	NoSubscriptions     string
	Save                string
	Unsubscribed        string
	UnsubscribedMessage string
}

// GetStrings returns localized strings based on locale
func GetStrings(locale string) Strings {
	// Extract base locale (e.g., "en" from "en-US")
	baseLocale := locale
	if len(locale) >= 2 {
		baseLocale = locale[:2]
	}

	switch baseLocale {
	case "es":
		return Strings{
			Title:               "Preferencias de Comunicación",
			Description:         "Elige qué métodos de comunicación deseas seguir recibiendo:",
			SuccessMessage:      "¡Tus preferencias han sido actualizadas!",
			NoSubscriptions:     "No estás suscrito a ninguna notificación.",
			Save:                "Guardar Preferencias",
			Unsubscribed:        "¡Te has dado de baja!",
			UnsubscribedMessage: "Has sido eliminado de esta lista de comunicaciones.",
		}
	case "fr":
		return Strings{
			Title:               "Préférences de communication",
			Description:         "Choisissez les méthodes de communication que vous souhaitez continuer à recevoir :",
			SuccessMessage:      "Vos préférences ont été mises à jour !",
			NoSubscriptions:     "Vous n'êtes abonné à aucune notification.",
			Save:                "Enregistrer les Préférences",
			Unsubscribed:        "Vous avez été désabonné !",
			UnsubscribedMessage: "Vous avez été retiré de cette liste de communications.",
		}
	case "de":
		return Strings{
			Title:               "Kommunikationspräferenzen",
			Description:         "Wählen Sie, welche Kommunikationsmethoden Sie weiterhin erhalten möchten:",
			SuccessMessage:      "Ihre Einstellungen wurden aktualisiert!",
			NoSubscriptions:     "Sie sind für keine Benachrichtigungen angemeldet.",
			Save:                "Einstellungen speichern",
			Unsubscribed:        "Sie wurden abgemeldet!",
			UnsubscribedMessage: "Sie wurden von dieser Kommunikationsliste entfernt.",
		}
	case "pt":
		return Strings{
			Title:               "Preferências de comunicação",
			Description:         "Escolha quais métodos de comunicação você gostaria de continuar recebendo:",
			SuccessMessage:      "Suas preferências foram atualizadas!",
			NoSubscriptions:     "Você não está inscrito em nenhuma notificação.",
			Save:                "Salvar Preferências",
			Unsubscribed:        "Você foi cancelado!",
			UnsubscribedMessage: "Você foi removido desta lista de comunicações.",
		}
	case "it":
		return Strings{
			Title:               "Preferenze di Iscrizione",
			Description:         "Scegli quali metodi di comunicazione desideri continuare a ricevere:",
			SuccessMessage:      "Le tue preferenze sono state aggiornate!",
			NoSubscriptions:     "Non sei iscritto a nessuna notifica.",
			Save:                "Salva Preferenze",
			Unsubscribed:        "Sei stato disiscritto!",
			UnsubscribedMessage: "Sei stato rimosso da questa lista di comunicazioni.",
		}
	default: // English
		return Strings{
			Title:               "Communication Preferences",
			Description:         "Choose which methods of communication you would like to continue to receive:",
			SuccessMessage:      "Your preferences have been updated!",
			NoSubscriptions:     "You are not subscribed to any notifications.",
			Save:                "Save Preferences",
			Unsubscribed:        "You have been unsubscribed!",
			UnsubscribedMessage: "You have been removed from this communication list.",
		}
	}
}

// UnsubscribeData holds data for the unsubscribe template
type UnsubscribeData struct {
	Locale  string
	Strings Strings
}

// RenderUnsubscribe renders the unsubscribe confirmation page
func RenderUnsubscribe(w io.Writer, locale string) error {
	data := UnsubscribeData{
		Locale:  locale,
		Strings: GetStrings(locale),
	}
	return unsubscribeTemplate.Execute(w, data)
}

// PreferencesData holds data for the preferences template
type PreferencesData struct {
	UserID             string
	Locale             string
	Strings            Strings
	Subscriptions      []oapi.UserSubscription
	ShowUpdatedMessage bool
}

// RenderPreferences renders the subscription preferences page
func RenderPreferences(w io.Writer, data PreferencesData) error {
	data.Strings = GetStrings(data.Locale)
	return preferencesTemplate.Execute(w, data)
}

// StaticFiles returns the embedded static files
func StaticFiles() embed.FS {
	return templateFiles
}
