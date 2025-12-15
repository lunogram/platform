package auth

import (
	"encoding/json"
	"net/http"
	"strings"
)

// OAuthResponse represents the OAuth token response stored in cookies
type OAuthResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// getCookieOAuthToken retrieves and parses the OAuth token from the 'oauth' cookie
func getCookieOAuthToken(r *http.Request) *OAuthResponse {
	cookie, err := r.Cookie("oauth")
	if err != nil {
		return nil
	}

	var oauthResp OAuthResponse
	if err := json.Unmarshal([]byte(cookie.Value), &oauthResp); err != nil {
		return nil
	}

	return &oauthResp
}

// RetrieveAuthToken extracts the authentication token from the request.
// It checks (in priority order):
// 1. OAuth access token from 'oauth' cookie
// 2. Session token from '__session' cookie
// 3. Authorization header (with or without Bearer prefix)
func RetrieveAuthToken(r *http.Request) string {
	if oauthToken := getCookieOAuthToken(r); oauthToken != nil && oauthToken.AccessToken != "" {
		return oauthToken.AccessToken
	}

	if sessionCookie, err := r.Cookie("__session"); err == nil && sessionCookie.Value != "" {
		return sessionCookie.Value
	}

	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		// Remove "Bearer " prefix if present
		token := strings.TrimSpace(authHeader)
		token = strings.TrimPrefix(token, "Bearer ")
		return token
	}

	return ""
}
