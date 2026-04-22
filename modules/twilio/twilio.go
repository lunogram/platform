package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"

	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	pdkhttp "github.com/extism/go-pdk/http"
	"github.com/lunogram/platform/modules/twilio/provider"
)

// Re-export exit codes from the provider package so the main module can use them directly.
const (
	ExitSuccess   = provider.ExitSuccess
	ExitTransient = provider.ExitTransient
	ExitPermanent = provider.ExitPermanent
)

// Config is an alias for provider.Config so the main module can use it
// without importing the provider package everywhere.
type Config = provider.Config

// safeTransport wraps an http.RoundTripper to guarantee that resp.Body is
// never nil. The standard http.Client contract promises a non-nil Body, but
// the Extism PDK transport can return nil when the response has no content.
type safeTransport struct {
	inner http.RoundTripper
}

func (t *safeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp != nil && resp.Body == nil {
		resp.Body = http.NoBody
	}
	return resp, nil
}

// CreateMessageParams holds the parameters for creating a Twilio message.
type CreateMessageParams struct {
	To             *string
	From           *string
	Body           *string
	StatusCallback *string
	MediaURLs      []string
}

func (p *CreateMessageParams) SetTo(to string) *CreateMessageParams {
	p.To = &to
	return p
}

func (p *CreateMessageParams) SetFrom(from string) *CreateMessageParams {
	p.From = &from
	return p
}

func (p *CreateMessageParams) SetBody(body string) *CreateMessageParams {
	p.Body = &body
	return p
}

func (p *CreateMessageParams) SetStatusCallback(cb string) *CreateMessageParams {
	p.StatusCallback = &cb
	return p
}

func (p *CreateMessageParams) SetMediaUrl(urls []string) *CreateMessageParams {
	p.MediaURLs = urls
	return p
}

// CreateMessageResponse represents a subset of the Twilio Messages API JSON response.
type CreateMessageResponse struct {
	Sid          *string `json:"sid"`
	Status       *string `json:"status"`
	ErrorCode    *int    `json:"error_code"`
	ErrorMessage *string `json:"error_message"`
}

// TwilioRestError represents an error response from the Twilio REST API.
// It intentionally does not implement the error interface to avoid triggering
// TinyGo's unimplemented AssignableTo reflection panic in encoding/json.
type TwilioRestError struct {
	Status   int    `json:"status"`
	Code     int    `json:"code"`
	Message  string `json:"message"`
	MoreInfo string `json:"more_info"`
}

// TwilioClient is a minimal Twilio REST API client that uses the Extism PDK
// HTTP transport for WASM compatibility.
type TwilioClient struct {
	accountSid string
	authToken  string
	httpClient *http.Client
}

// NewTwilioClient creates a TwilioClient configured with the Extism PDK
// HTTP transport.
func NewTwilioClient(accountSid, authToken string) *TwilioClient {
	return &TwilioClient{
		accountSid: accountSid,
		authToken:  authToken,
		httpClient: &http.Client{
			Transport: &safeTransport{inner: &pdkhttp.HTTPTransport{}},
			Timeout:   30 * time.Second,
		},
	}
}

// SendResult holds the outcome of a CreateMessage call. On success, Response
// is populated. On failure, Err is set and HTTPStatus contains the status code
// returned by the Twilio API (0 if the request never reached Twilio).
type SendResult struct {
	Response   *CreateMessageResponse
	Err        error
	HTTPStatus int
}

// CreateMessage sends an SMS/MMS via the Twilio Messages API.
func (c *TwilioClient) CreateMessage(params *CreateMessageParams) SendResult {
	form := url.Values{}
	if params.To != nil {
		form.Set("To", *params.To)
	}
	if params.From != nil {
		form.Set("From", *params.From)
	}
	if params.Body != nil {
		form.Set("Body", *params.Body)
	}
	if params.StatusCallback != nil {
		form.Set("StatusCallback", *params.StatusCallback)
	}
	for _, u := range params.MediaURLs {
		form.Add("MediaUrl", u)
	}

	endpoint := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", c.accountSid)

	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return SendResult{Err: fmt.Errorf("failed to create request: %w", err)}
	}
	req.SetBasicAuth(c.accountSid, c.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return SendResult{Err: fmt.Errorf("failed to execute request: %w", err)}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return SendResult{HTTPStatus: resp.StatusCode, Err: fmt.Errorf("failed to read response body: %w", err)}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		restErr := TwilioRestError{Status: resp.StatusCode}
		if jsonErr := json.Unmarshal(body, &restErr); jsonErr != nil {
			restErr.Message = string(body)
		}
		restErr.Status = resp.StatusCode
		return SendResult{
			HTTPStatus: resp.StatusCode,
			Err:        fmt.Errorf("twilio API error (status=%d, code=%d): %s", restErr.Status, restErr.Code, restErr.Message),
		}
	}

	var result CreateMessageResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return SendResult{HTTPStatus: resp.StatusCode, Err: fmt.Errorf("failed to decode response JSON: %w", err)}
	}
	return SendResult{Response: &result, HTTPStatus: resp.StatusCode}
}

// RequestValidator validates incoming Twilio webhook requests using HMAC-SHA1
// signature verification.
type RequestValidator struct {
	signingKey []byte
}

// NewRequestValidator creates a RequestValidator with the given auth token.
func NewRequestValidator(authToken string) RequestValidator {
	return RequestValidator{signingKey: []byte(authToken)}
}

// Validate checks the Twilio request signature. It tries the URL as-is first,
// then with and without the port to handle proxy/load-balancer differences.
func (rv RequestValidator) Validate(requestURL string, params map[string]string, expectedSignature string) bool {
	// Try the URL as provided.
	if rv.compare(rv.buildSignature(requestURL, params), expectedSignature) {
		return true
	}

	// Try with port added (e.g. :443 for https, :80 for http).
	withPort := addPort(requestURL)
	if withPort != requestURL {
		if rv.compare(rv.buildSignature(withPort, params), expectedSignature) {
			return true
		}
	}

	// Try with port removed.
	withoutPort := removePort(requestURL)
	if withoutPort != requestURL {
		if rv.compare(rv.buildSignature(withoutPort, params), expectedSignature) {
			return true
		}
	}

	return false
}

// buildSignature computes the HMAC-SHA1 signature for the given URL and params.
func (rv RequestValidator) buildSignature(requestURL string, params map[string]string) string {
	// Sort the param keys.
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build the data string: URL + sorted key-value pairs concatenated.
	var buf strings.Builder
	buf.WriteString(requestURL)
	for _, k := range keys {
		buf.WriteString(k)
		buf.WriteString(params[k])
	}

	mac := hmac.New(sha1.New, rv.signingKey)
	mac.Write([]byte(buf.String()))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// compare performs a constant-time comparison of two signature strings.
func (rv RequestValidator) compare(computed, expected string) bool {
	return subtle.ConstantTimeCompare([]byte(computed), []byte(expected)) == 1
}

// addPort adds the default port to a URL if it doesn't already have one.
func addPort(rawURL string) string {
	return updatePort(rawURL, true)
}

// removePort removes the port from a URL if present.
func removePort(rawURL string) string {
	return updatePort(rawURL, false)
}

// updatePort either adds or removes the default port from a URL.
func updatePort(rawURL string, add bool) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	host := parsed.Hostname()
	port := parsed.Port()

	if add {
		if port != "" {
			// Already has a port, nothing to add.
			return rawURL
		}
		defaultPort := "443"
		if parsed.Scheme == "http" {
			defaultPort = "80"
		}
		parsed.Host = host + ":" + defaultPort
	} else {
		if port == "" {
			// No port to remove.
			return rawURL
		}
		parsed.Host = host
	}

	return parsed.String()
}
