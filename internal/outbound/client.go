package outbound

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/cenkalti/backoff/v5"
	"go.uber.org/zap"
)

// Request is one outbound call. It is a value rather than an *http.Request
// because retries need to rebuild the request per attempt, and because keeping
// the body as bytes makes a retried request byte-identical to the first.
type Request struct {
	Method string
	URL    string
	Body   []byte
	Header http.Header
	Query  url.Values

	// IgnoreStatus makes any completed exchange a success, whatever the status.
	// Without it a 5xx is retried and a 4xx is an error; with it the first
	// answer the destination gives is the answer, and only a transport failure
	// is retried. It exists for callers that were told not to look at the
	// response at all — retrying a status nobody will read only spends the
	// dispatch budget.
	IgnoreStatus bool
}

// Response is the result of a completed call. Body is bounded by the client's
// MaxResponseBytes.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// StatusError is returned when a destination answers with a non-2xx status. It
// carries the truncated body so a caller that wants to surface upstream detail
// can, without every caller having to read the body itself.
type StatusError struct {
	StatusCode int
	Body       []byte
}

// Error implements error. The body is deliberately not interpolated: it is
// attacker-influenced content from an operator-configured endpoint and belongs
// in a structured log field, not in an error string that may be returned to an
// API caller.
func (e *StatusError) Error() string {
	return fmt.Sprintf("outbound: destination returned status %d", e.StatusCode)
}

// Options configures a [Client].
type Options struct {
	Timeout          time.Duration
	Network          Network
	Auth             AuthConfig
	Retry            Retry
	MaxResponseBytes int64
	Logger           *zap.Logger
}

// Client performs guarded, authenticated, retrying outbound HTTP calls to a
// single operator-configured destination. It is the transport primitive shared
// by the webhook engine and by any other outbound integration; it knows nothing
// about events or hooks.
type Client struct {
	http     *http.Client
	auth     Strategy
	retry    Retry
	maxBytes int64
	logger   *zap.Logger
}

// NewClient builds a client for one destination. The SSRF policy is baked into
// the returned client's dialer, so a client built for a strict destination
// cannot later be reused for a relaxed one.
func NewClient(opts Options) (*Client, error) {
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	httpClient := ssrfClient(timeout, opts.Network)

	auth, err := BuildStrategy(opts.Auth, StrategyDeps{HTTPClient: httpClient})
	if err != nil {
		return nil, err
	}

	retry := opts.Retry.WithDefaults(DefaultRetry(), timeout)
	if err := retry.Validate(); err != nil {
		return nil, err
	}

	maxBytes := opts.MaxResponseBytes
	if maxBytes <= 0 {
		maxBytes = DefaultMaxResponseBytes
	}

	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Client{http: httpClient, auth: auth, retry: retry, maxBytes: maxBytes, logger: logger}, nil
}

// Do performs req, retrying retryable failures with capped exponential backoff.
// It honours ctx throughout: a cancelled or expired context ends the attempt
// sequence immediately rather than sleeping out the remaining backoff.
func (c *Client) Do(ctx context.Context, req Request) (*Response, error) {
	strategy := backoff.NewExponentialBackOff()
	strategy.InitialInterval = c.retry.InitialInterval
	strategy.MaxInterval = c.retry.MaxInterval

	attempt := 0
	operation := func() (*Response, error) {
		attempt++
		resp, err := c.attempt(ctx, req)
		if err != nil {
			// A context that is already done will never succeed on a retry, and
			// backoff only checks the context after the sleep is scheduled.
			if ctx.Err() != nil {
				return nil, backoff.Permanent(err)
			}
			return nil, err
		}

		if req.IgnoreStatus || resp.StatusCode < 400 {
			return resp, nil
		}

		statusErr := &StatusError{StatusCode: resp.StatusCode, Body: resp.Body}
		if !Retryable(resp.StatusCode) {
			return nil, backoff.Permanent(statusErr)
		}
		if after, ok := retryAfter(resp.Header); ok {
			return nil, &retryAfterStatus{status: statusErr, after: backoff.RetryAfter(int(after.Seconds()))}
		}
		return nil, statusErr
	}

	opts := []backoff.RetryOption{
		backoff.WithBackOff(strategy),
		backoff.WithMaxTries(uint(c.retry.MaxAttempts)), //nolint:gosec // validated >= 1
		backoff.WithNotify(func(err error, next time.Duration) {
			c.logger.Warn("outbound request failed, retrying",
				zap.Int("attempt", attempt),
				zap.Duration("retry_in", next),
				zap.Error(err),
			)
		}),
	}
	if c.retry.MaxElapsedTime > 0 {
		opts = append(opts, backoff.WithMaxElapsedTime(c.retry.MaxElapsedTime))
	}

	resp, err := backoff.Retry(ctx, operation, opts...)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// attempt performs a single call, building a fresh *http.Request so the auth
// strategy re-runs (and an OAuth2 token can be refreshed) on every attempt.
func (c *Client) attempt(ctx context.Context, req Request) (*Response, error) {
	var body io.Reader
	if len(req.Body) > 0 {
		body = bytes.NewReader(req.Body)
	}

	method := req.Method
	if method == "" {
		method = http.MethodPost
	}

	httpReq, err := http.NewRequestWithContext(ctx, method, req.URL, body)
	if err != nil {
		return nil, fmt.Errorf("outbound: build request: %w", err)
	}
	if len(req.Query) > 0 {
		httpReq.URL.RawQuery = req.Query.Encode()
	}
	for key, values := range req.Header {
		for _, value := range values {
			httpReq.Header.Add(key, value)
		}
	}

	// Auth is applied last so a configured credential always wins over a header
	// that reached the request by another route (a forwarded inbound header).
	if c.auth != nil {
		if err := c.auth.Apply(ctx, httpReq); err != nil {
			return nil, err
		}
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("outbound: request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	payload, err := io.ReadAll(io.LimitReader(resp.Body, c.maxBytes))
	if err != nil {
		return nil, fmt.Errorf("outbound: read response: %w", err)
	}
	// Drain whatever is left so the connection can be reused, but never beyond
	// the cap — a destination that streams forever must not hold us open.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, c.maxBytes))

	return &Response{StatusCode: resp.StatusCode, Header: resp.Header, Body: payload}, nil
}

// retryAfterStatus carries both the destination's status and its Retry-After
// hint. backoff reads the hint through errors.As to schedule the next attempt;
// callers read the status through the same mechanism once attempts run out, so
// honouring Retry-After does not cost them the status they were told about.
type retryAfterStatus struct {
	status *StatusError
	after  error
}

func (e *retryAfterStatus) Error() string   { return e.status.Error() }
func (e *retryAfterStatus) Unwrap() []error { return []error{e.status, e.after} }

// Retryable reports whether a status warrants another attempt. Server errors
// and the two "come back later" client errors are retried; every other 4xx is
// a request the destination will reject identically next time, so retrying it
// only multiplies the latency a caller waits through.
func Retryable(status int) bool {
	switch status {
	case http.StatusRequestTimeout, http.StatusTooManyRequests:
		return true
	}
	return status >= 500
}

// retryAfter parses a Retry-After header expressed in seconds. The HTTP-date
// form is ignored: it is rare in practice, and a misparsed date would be worse
// than falling back to the configured backoff.
func retryAfter(header http.Header) (time.Duration, bool) {
	raw := header.Get("Retry-After")
	if raw == "" {
		return 0, false
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds < 0 {
		return 0, false
	}
	return time.Duration(seconds) * time.Second, true
}

// AsStatusError extracts a [StatusError] from err, if there is one.
func AsStatusError(err error) (*StatusError, bool) {
	var statusErr *StatusError
	if errors.As(err, &statusErr) {
		return statusErr, true
	}
	return nil, false
}
