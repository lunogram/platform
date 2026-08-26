package outbound

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

func init() {
	RegisterStrategy("oauth2_client_credentials", buildOAuth2ClientCredentials)
}

// tokenExpiryLeeway is subtracted from a token's lifetime before it is
// considered expired, so a token is never presented to the destination in the
// window where it is about to lapse in flight.
const tokenExpiryLeeway = 30 * time.Second

type oauth2Config struct {
	TokenURL     string   `yaml:"token_url"`
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	Scopes       []string `yaml:"scopes"`
}

// OAuth2ClientCredentials fetches a bearer token with the client-credentials
// grant and caches it until shortly before it expires. The token endpoint is
// called through the destination's own guarded client, so it is subject to the
// same SSRF policy as the destination itself.
type OAuth2ClientCredentials struct {
	tokenURL     string
	clientID     string
	clientSecret string
	scopes       []string
	client       *http.Client

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

func buildOAuth2ClientCredentials(node yaml.Node, deps StrategyDeps) (Strategy, error) {
	var cfg oauth2Config
	if err := decodeStrategyConfig(node, &cfg); err != nil {
		return nil, err
	}
	if cfg.TokenURL == "" {
		return nil, fmt.Errorf("token_url is required")
	}
	if cfg.ClientID == "" {
		return nil, fmt.Errorf("client_id is required")
	}
	if cfg.ClientSecret == "" {
		return nil, fmt.Errorf("client_secret is required")
	}
	client := deps.HTTPClient
	if client == nil {
		return nil, fmt.Errorf("no http client available for the token endpoint")
	}
	return &OAuth2ClientCredentials{
		tokenURL:     cfg.TokenURL,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		scopes:       cfg.Scopes,
		client:       client,
	}, nil
}

// Apply implements [Strategy].
func (o *OAuth2ClientCredentials) Apply(ctx context.Context, req *http.Request) error {
	token, err := o.accessToken(ctx)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return nil
}

// accessToken returns a cached token when one is still comfortably valid, and
// otherwise fetches a fresh one. The lock is held across the fetch so a burst
// of concurrent dispatches produces one token request rather than one per hook.
func (o *OAuth2ClientCredentials) accessToken(ctx context.Context) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if o.token != "" && time.Now().Before(o.expiresAt) {
		return o.token, nil
	}

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	if len(o.scopes) > 0 {
		form.Set("scope", strings.Join(o.scopes, " "))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("oauth2: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(url.QueryEscape(o.clientID), url.QueryEscape(o.clientSecret))

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("oauth2: token request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	body, err := io.ReadAll(io.LimitReader(resp.Body, DefaultMaxResponseBytes))
	if err != nil {
		return "", fmt.Errorf("oauth2: read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// The body may echo the client_id and any error detail; it is not
		// included in the error so a token-endpoint misconfiguration cannot
		// spill credentials into the logs.
		return "", fmt.Errorf("oauth2: token endpoint returned status %d", resp.StatusCode)
	}

	var payload struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", fmt.Errorf("oauth2: invalid token response: %w", err)
	}
	if payload.AccessToken == "" {
		return "", fmt.Errorf("oauth2: token response contained no access_token")
	}

	lifetime := time.Duration(payload.ExpiresIn) * time.Second
	if lifetime > tokenExpiryLeeway {
		o.expiresAt = time.Now().Add(lifetime - tokenExpiryLeeway)
	} else {
		// A token with no or a very short stated lifetime is used once and
		// re-fetched, rather than cached past a lifetime we cannot trust.
		o.expiresAt = time.Time{}
	}
	o.token = payload.AccessToken

	return o.token, nil
}
