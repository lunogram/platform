package outbound

import (
	"context"
	"fmt"
	"net/http"

	"gopkg.in/yaml.v3"
)

func init() {
	RegisterStrategy("api_key", buildAPIKey)
	RegisterStrategy("basic_auth", buildBasicAuth)
}

// APIKeyIn selects where an API key is placed on the request.
type APIKeyIn string

const (
	APIKeyInHeader APIKeyIn = "header"
	APIKeyInCookie APIKeyIn = "cookie"
)

type apiKeyConfig struct {
	In    APIKeyIn `yaml:"in"`
	Name  string   `yaml:"name"`
	Value string   `yaml:"value"`
}

// APIKey injects a static credential into a named header or cookie.
type APIKey struct {
	in    APIKeyIn
	name  string
	value string
}

func buildAPIKey(node yaml.Node, _ StrategyDeps) (Strategy, error) {
	var cfg apiKeyConfig
	if err := decodeStrategyConfig(node, &cfg); err != nil {
		return nil, err
	}
	if cfg.In == "" {
		cfg.In = APIKeyInHeader
	}
	if cfg.In != APIKeyInHeader && cfg.In != APIKeyInCookie {
		return nil, fmt.Errorf("in must be %q or %q, got %q", APIKeyInHeader, APIKeyInCookie, cfg.In)
	}
	if cfg.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if cfg.Value == "" {
		return nil, fmt.Errorf("value is required")
	}
	return &APIKey{in: cfg.In, name: cfg.Name, value: cfg.Value}, nil
}

// Apply implements [Strategy].
func (a *APIKey) Apply(_ context.Context, req *http.Request) error {
	switch a.in {
	case APIKeyInCookie:
		req.AddCookie(&http.Cookie{Name: a.name, Value: a.value})
	default:
		req.Header.Set(a.name, a.value)
	}
	return nil
}

type basicAuthConfig struct {
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

// BasicAuth sets an HTTP basic Authorization header.
type BasicAuth struct {
	user     string
	password string
}

func buildBasicAuth(node yaml.Node, _ StrategyDeps) (Strategy, error) {
	var cfg basicAuthConfig
	if err := decodeStrategyConfig(node, &cfg); err != nil {
		return nil, err
	}
	if cfg.User == "" {
		return nil, fmt.Errorf("user is required")
	}
	if cfg.Password == "" {
		return nil, fmt.Errorf("password is required")
	}
	return &BasicAuth{user: cfg.User, password: cfg.Password}, nil
}

// Apply implements [Strategy].
func (b *BasicAuth) Apply(_ context.Context, req *http.Request) error {
	req.SetBasicAuth(b.user, b.password)
	return nil
}
