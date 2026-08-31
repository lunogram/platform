package http

import "time"

// DefaultConfig returns the server timeouts and CSRF settings used when nothing
// configures them.
func DefaultConfig() Config {
	return Config{
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		IdleTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		MaxHeaderBytes:    1048576,
		CSRF: CSRFConfig{
			Enabled:        true,
			TokenLength:    32,
			CookieName:     "csrf_token",
			HeaderName:     "X-CSRF-Token",
			FieldName:      "csrf_token",
			CookieSecure:   true,
			CookieHTTPOnly: true,
			CookieSameSite: "strict",
		},
	}
}
