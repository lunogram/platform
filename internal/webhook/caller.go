package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/oapi"
	"go.uber.org/zap"
)

// Caller handles webhook delivery.
type Caller struct {
	logger                *zap.Logger
	projectCreatedURL     string
	projectCreatedTimeout time.Duration
	emailTemplatesURL     string
	client                *http.Client
	emailTemplatesClient  *http.Client
}

// NewCaller creates a new webhook caller.
func NewCaller(logger *zap.Logger, cfg config.Webhook) *Caller {
	timeout := cfg.ProjectCreatedTimeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	emailTimeout := cfg.EmailTemplatesTimeout
	if emailTimeout == 0 {
		emailTimeout = 10 * time.Second
	}

	return &Caller{
		logger:                logger,
		projectCreatedURL:     cfg.ProjectCreatedURL,
		projectCreatedTimeout: timeout,
		emailTemplatesURL:     cfg.EmailTemplatesURL,
		client: &http.Client{
			Timeout: timeout,
		},
		emailTemplatesClient: &http.Client{
			Timeout: emailTimeout,
		},
	}
}

// Enabled returns true if any webhook is configured.
func (c *Caller) Enabled() bool {
	return c.projectCreatedURL != "" || c.emailTemplatesURL != ""
}

// EmailTemplatesEnabled returns true if the email templates webhook is configured.
func (c *Caller) EmailTemplatesEnabled() bool {
	if c == nil {
		return false
	}
	return c.emailTemplatesURL != ""
}

// ProjectCreated sends a webhook for a newly created project.
// The request headers are forwarded to the webhook endpoint.
// This method is safe to call on a nil receiver or if no webhook URL is configured - it will return nil.
func (c *Caller) ProjectCreated(ctx context.Context, r *http.Request, project oapi.ProjectDetails) error {
	if c == nil || c.projectCreatedURL == "" {
		return nil
	}

	logger := c.logger.With(
		zap.Stringer("project_id", project.Id),
		zap.Stringer("organization_id", project.OrganizationId),
	)

	payload := oapi.ProjectCreatedEvent{
		Event:     oapi.ProjectCreated,
		Timestamp: time.Now().UTC(),
		Project:   project,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal webhook payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.projectCreatedURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create webhook request: %w", err)
	}

	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Event", string(payload.Event))

	logger.Info("calling project created webhook")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	// Drain the response body to allow HTTP connection reuse
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned error status: %d", resp.StatusCode)
	}

	logger.Info("project created webhook delivered", zap.Int("status_code", resp.StatusCode))
	return nil
}

// EmailTemplates proxies a GET request to the configured email templates webhook.
// It forwards the original request headers and query parameters.
// Returns the raw response body bytes, or nil if no webhook is configured.
func (c *Caller) EmailTemplates(ctx context.Context, r *http.Request) ([]byte, error) {
	if c == nil || c.emailTemplatesURL == "" {
		return nil, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.emailTemplatesURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create email templates request: %w", err)
	}

	// Forward query parameters
	req.URL.RawQuery = r.URL.RawQuery

	// Forward headers for authentication
	for key, values := range r.Header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	req.Header.Set("Accept", "application/json")

	c.logger.Info("calling email templates webhook")

	resp, err := c.emailTemplatesClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("email templates webhook request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read email templates webhook response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("email templates webhook returned error status: %d", resp.StatusCode)
	}

	c.logger.Info("email templates webhook response received", zap.Int("status_code", resp.StatusCode))
	return body, nil
}
