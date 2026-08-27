// Package gallery fetches the email starter-template gallery from an
// operator-configured endpoint.
//
// This is deliberately not a webhook. Fetching the gallery is a synchronous GET
// whose response body is the handler's return value: there is exactly one
// endpoint, it must exist for the request to mean anything, its response must
// be parsed, and its failure must fail the request. Every degree of freedom the
// hook model offers — many subscribers, best-effort delivery, a rendered body,
// an ignored response — would be pinned to a single legal value, which is the
// sign that it is not a hook.
//
// What it does share with hooks is the part that is genuinely common: guarded,
// authenticated, retrying transport. That lives in internal/outbound and is
// used directly here.
package gallery

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/lunogram/platform/internal/outbound"
	"github.com/lunogram/platform/internal/webhook"
	"go.uber.org/zap"
)

// Query narrows a gallery listing. The fields mirror the console's list
// parameters; they are re-emitted explicitly rather than by forwarding the
// inbound query string, so a caller cannot smuggle arbitrary parameters through
// the platform to the configured endpoint.
type Query struct {
	Limit  *int
	Offset *int
	Search *string
}

// Template is one starter template as the gallery describes it.
//
// This is the package's own type rather than the management API's generated
// one. The gallery is a domain client; making it speak the HTTP layer's wire
// format would point the dependency the wrong way through the architecture and
// couple a transport concern to a controller's generated types. Translating to
// the API response is the controller's job.
type Template struct {
	ID          string          `json:"id"`
	Label       string          `json:"label"`
	Description *string         `json:"description,omitempty"`
	HTML        *string         `json:"html,omitempty"`
	Text        *string         `json:"text,omitempty"`
	Thumbnail   *string         `json:"thumbnail,omitempty"`
	Blocks      *map[string]any `json:"blocks,omitempty"`
}

// Listing is a page of starter templates.
type Listing struct {
	Total   int        `json:"total"`
	Limit   int        `json:"limit"`
	Offset  int        `json:"offset"`
	Results []Template `json:"results"`
}

// Client fetches starter templates.
type Client struct {
	url    string
	client *outbound.Client
	logger *zap.Logger
}

// New builds a gallery client, or returns nil when no gallery is configured. A
// nil client is usable and reports itself disabled.
func New(logger *zap.Logger, cfg *webhook.GalleryConfig) (*Client, error) {
	if cfg == nil || cfg.URL == "" {
		return nil, nil
	}

	if err := outbound.ValidateURL(cfg.URL, cfg.Network); err != nil {
		return nil, fmt.Errorf("email templates: %w", err)
	}

	retry := outbound.Retry{}
	if cfg.Retry != nil {
		retry = *cfg.Retry
	}
	retry = retry.WithDefaults(outbound.DefaultRetry(), cfg.Timeout)

	client, err := outbound.NewClient(outbound.Options{
		Timeout:          cfg.Timeout,
		Network:          cfg.Network,
		Auth:             cfg.Auth,
		Retry:            retry,
		MaxResponseBytes: cfg.MaxResponseBytes,
		Logger:           logger,
	})
	if err != nil {
		return nil, fmt.Errorf("email templates: %w", err)
	}

	return &Client{url: cfg.URL, client: client, logger: logger}, nil
}

// Enabled reports whether a gallery endpoint is configured.
func (c *Client) Enabled() bool {
	return c != nil && c.url != ""
}

// List fetches a page of the gallery.
//
// The response is decoded rather than proxied through. The endpoint is
// operator-configured and its payload is rendered in the console, so passing
// its bytes through unread would let whoever controls that URL choose what the
// console receives. Decoding bounds the body, rejects anything that is not the
// documented shape, and drops fields the schema does not know about — a new
// gallery field needs a spec bump to surface anyway.
func (c *Client) List(ctx context.Context, query Query) (*Listing, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("email templates: no gallery configured")
	}

	values := url.Values{}
	if query.Limit != nil {
		values.Set("limit", strconv.Itoa(*query.Limit))
	}
	if query.Offset != nil {
		values.Set("offset", strconv.Itoa(*query.Offset))
	}
	if query.Search != nil && *query.Search != "" {
		values.Set("search", *query.Search)
	}

	resp, err := c.client.Do(ctx, outbound.Request{
		Method: http.MethodGet,
		URL:    c.url,
		Query:  values,
		Header: http.Header{"Accept": []string{"application/json"}},
	})
	if err != nil {
		return nil, fmt.Errorf("email templates: %w", err)
	}

	listing := Listing{Results: []Template{}}
	if err := json.Unmarshal(resp.Body, &listing); err != nil {
		return nil, fmt.Errorf("email templates: invalid gallery response: %w", err)
	}
	return &listing, nil
}
