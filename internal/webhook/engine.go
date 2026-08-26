package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/outbound"
	"github.com/lunogram/platform/internal/rbac"
	"go.uber.org/zap"
)

// Engine dispatches registered events to the hooks bound to them in
// configuration. The zero value and a nil *Engine are both usable and dispatch
// nothing, so a deployment with no hooks configured needs no call-site guards.
type Engine struct {
	hooks    map[string][]*hook
	budget   time.Duration
	logger   *zap.Logger
	gallery  *GalleryConfig
	warnings []string
}

// hook is one compiled subscriber: config resolved, template parsed, transport
// and credentials built. Everything that can fail on a well-formed request has
// already failed by the time a hook exists.
type hook struct {
	id           string
	event        string
	url          string
	method       string
	template     *Template
	client       *outbound.Client
	canInterrupt bool
	response     ResponseConfig
	forward      []string
}

// Result is the outcome of one hook. Body is populated only for hooks
// configured with response.parse.
type Result struct {
	HookID     string
	StatusCode int
	Body       json.RawMessage
	Err        error
}

// Results are the outcomes of every hook that ran for one dispatch, in the
// order they were declared.
type Results []Result

// Parsed returns the parsed response body of a named hook.
func (r Results) Parsed(hookID string) (json.RawMessage, bool) {
	for _, result := range r {
		if result.HookID == hookID && result.Body != nil {
			return result.Body, true
		}
	}
	return nil, false
}

// Actor is the authenticated identity that triggered an event. It travels in
// the envelope so a receiver can attribute the event without the engine having
// to forward the caller's credential to do it.
type Actor struct {
	Type           string `json:"type"`
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	ProjectID      string `json:"project_id"`
}

// Envelope is the context handed to every body template as `ctx`. Its shape is
// part of the operator-facing contract: templates index into it by name.
type Envelope struct {
	Event      string    `json:"event"`
	Version    string    `json:"version"`
	OccurredAt time.Time `json:"occurred_at"`
	Actor      Actor     `json:"actor"`
	Payload    any       `json:"payload"`
}

type dispatchOptions struct {
	inbound *http.Request
}

// DispatchOption adjusts a single dispatch.
type DispatchOption func(*dispatchOptions)

// WithInboundRequest makes the triggering HTTP request's headers available for
// forwarding.
//
// Deprecated: forwarding is only performed for headers a hook names in its
// deprecated forward_headers allowlist, and it requires this option at the call
// site as well. Two places must agree before a caller's credential reaches an
// operator-configured URL, and both are greppable. New call sites should not
// use it; configure the hook's own auth instead.
func WithInboundRequest(r *http.Request) DispatchOption {
	return func(o *dispatchOptions) { o.inbound = r }
}

// New compiles a configuration into an engine. Every template is parsed, every
// URL validated against its network policy, and every credential built here, so
// a misconfiguration fails at boot rather than on the first event.
func New(cfg *Config, logger *zap.Logger) (*Engine, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	if cfg == nil {
		return &Engine{budget: DefaultMaxDispatchTime, logger: logger}, nil
	}

	engine := &Engine{
		hooks:   map[string][]*hook{},
		budget:  cfg.maxDispatchTime(),
		logger:  logger,
		gallery: cfg.EmailTemplates,
	}

	events := make([]string, 0, len(cfg.Hooks))
	for event := range cfg.Hooks {
		events = append(events, event)
	}
	sort.Strings(events)

	for _, event := range events {
		if !IsRegistered(event) {
			return nil, fmt.Errorf("webhook config: unknown event %q (known: %v)", event, Registered())
		}

		seen := map[string]bool{}
		for index, hookCfg := range cfg.Hooks[event] {
			if hookCfg.ID == "" {
				return nil, fmt.Errorf("webhook config: %s hook %d: id is required", event, index)
			}
			if seen[hookCfg.ID] {
				return nil, fmt.Errorf("webhook config: %s: duplicate hook id %q", event, hookCfg.ID)
			}
			seen[hookCfg.ID] = true

			compiled, warnings, err := engine.compile(cfg, event, hookCfg)
			if err != nil {
				return nil, fmt.Errorf("webhook config: %s hook %q: %w", event, hookCfg.ID, err)
			}
			engine.warnings = append(engine.warnings, warnings...)
			engine.hooks[event] = append(engine.hooks[event], compiled)
		}
	}

	for _, warning := range engine.warnings {
		logger.Warn(warning)
	}

	return engine, nil
}

// compile resolves one hook's configuration into a runnable hook, returning any
// operator-facing warnings it should be told about at boot.
func (e *Engine) compile(cfg *Config, event string, hookCfg HookConfig) (*hook, []string, error) {
	var warnings []string

	if hookCfg.URL == "" {
		return nil, nil, errors.New("url is required")
	}

	network := cfg.resolvedNetwork(hookCfg)
	if err := outbound.ValidateURL(hookCfg.URL, network); err != nil {
		return nil, nil, err
	}
	if network.Relaxed() {
		warnings = append(warnings, fmt.Sprintf(
			"webhook %s/%s relaxes SSRF protection (allow_private=%t allow_http=%t); its url is not restricted to public https addresses",
			event, hookCfg.ID, network.AllowPrivate, network.AllowHTTP))
	}

	response := cfg.resolvedResponse(hookCfg)
	if response.Ignore && response.Parse {
		return nil, nil, errors.New("response.ignore and response.parse are mutually exclusive")
	}
	if response.Parse && !hookCfg.CanInterrupt {
		warnings = append(warnings, fmt.Sprintf(
			"webhook %s/%s parses its response but cannot interrupt; the parsed body is available to the caller but a failure will not stop the operation",
			event, hookCfg.ID))
	}

	timeout := cfg.resolvedTimeout(hookCfg)
	if timeout > e.budget {
		return nil, nil, fmt.Errorf("timeout %s exceeds the dispatch budget %s", timeout, e.budget)
	}

	retry := cfg.resolvedRetry(hookCfg, timeout)
	if err := retry.Validate(); err != nil {
		return nil, nil, err
	}
	// A hook may not budget more retrying than the whole dispatch is allowed to
	// take. Clamping here rather than rejecting keeps a generous global retry
	// default from making every short-timeout hook a config error.
	if retry.MaxElapsedTime > e.budget {
		retry.MaxElapsedTime = e.budget
	}

	template, err := resolveTemplate(event, hookCfg, cfg.baseDir)
	if err != nil {
		return nil, nil, err
	}

	client, err := outbound.NewClient(outbound.Options{
		Timeout:          timeout,
		Network:          network,
		Auth:             hookCfg.Auth,
		Retry:            retry,
		MaxResponseBytes: hookCfg.MaxResponseBytes,
		Logger:           e.logger.With(zap.String("event", event), zap.String("hook", hookCfg.ID)),
	})
	if err != nil {
		return nil, nil, err
	}

	forward := make([]string, 0, len(hookCfg.ForwardHeaders))
	for _, name := range hookCfg.ForwardHeaders {
		forward = append(forward, http.CanonicalHeaderKey(name))
	}
	if len(forward) > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"webhook %s/%s forwards inbound request headers %v to %s; this is deprecated and hands the caller's credentials to the configured url",
			event, hookCfg.ID, forward, hookCfg.URL))
	}

	method := hookCfg.Method
	if method == "" {
		method = http.MethodPost
	}

	return &hook{
		id:           hookCfg.ID,
		event:        event,
		url:          hookCfg.URL,
		method:       method,
		template:     template,
		client:       client,
		canInterrupt: hookCfg.CanInterrupt,
		response:     response,
		forward:      forward,
	}, warnings, nil
}

// resolveTemplate picks the hook's body template, falling back to the event's
// embedded default when the hook does not specify one.
func resolveTemplate(event string, hookCfg HookConfig, baseDir string) (*Template, error) {
	if hookCfg.Body != "" {
		return ParseTemplate(event+"/"+hookCfg.ID, hookCfg.Body, baseDir)
	}
	return defaultTemplate(event)
}

// Enabled reports whether any hook is bound to the given event. Call sites use
// it only to skip building a payload that nothing will consume; Dispatch is
// safe to call regardless.
func (e *Engine) Enabled(event string) bool {
	if e == nil {
		return false
	}
	return len(e.hooks[event]) > 0
}

// Gallery returns the email template gallery configuration, if the file carries
// one.
func (e *Engine) Gallery() *GalleryConfig {
	if e == nil {
		return nil
	}
	return e.gallery
}

// Dispatch fires every hook bound to the event, in the order they are declared.
//
// Hooks run sequentially. Concurrency would shave latency when several hooks
// are bound to one event, but it would also make partial-failure states
// non-deterministic and interleave the logs of hooks that operators reason
// about as an ordered list. The common case is one hook, where concurrency buys
// nothing.
//
// The first failure of a hook with can_interrupt stops the remaining hooks and
// is returned. Once the originating operation is going to be reported as
// failed, firing further side effects for it is worse than not firing them.
// Hooks without can_interrupt never stop anything: their failures are logged,
// recorded in the results, and swallowed.
//
// The whole dispatch is bounded by the configured max_dispatch_time regardless
// of how many hooks are bound or how they are configured to retry, so the
// latency a caller can experience is one number rather than a product.
func (e *Engine) Dispatch(ctx context.Context, event Event, opts ...DispatchOption) (Results, error) {
	if e == nil || event.definition == nil {
		return nil, nil
	}
	hooks := e.hooks[event.definition.Name]
	if len(hooks) == 0 {
		return nil, nil
	}

	options := dispatchOptions{}
	for _, opt := range opts {
		opt(&options)
	}

	envelope := Envelope{
		Event:      event.definition.Name,
		Version:    event.definition.Version,
		OccurredAt: event.occurredAt,
		Actor:      actorFrom(ctx),
		Payload:    event.payload,
	}
	envelopeJSON, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("webhook: marshal event context: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, e.budget)
	defer cancel()

	results := make(Results, 0, len(hooks))
	for _, h := range hooks {
		result := h.run(ctx, e.logger, envelopeJSON, options.inbound)
		results = append(results, result)

		if result.Err == nil {
			continue
		}
		if !h.canInterrupt {
			e.logger.Error("webhook delivery failed",
				zap.String("event", h.event),
				zap.String("hook", h.id),
				zap.Int("status_code", result.StatusCode),
				zap.Error(result.Err),
			)
			continue
		}
		return results, fmt.Errorf("webhook %s/%s: %w", h.event, h.id, result.Err)
	}

	return results, nil
}

// run performs one hook and never panics on a nil response.
func (h *hook) run(ctx context.Context, logger *zap.Logger, envelopeJSON []byte, inbound *http.Request) Result {
	result := Result{HookID: h.id}

	body, err := h.template.Render(envelopeJSON)
	if err != nil {
		result.Err = err
		return result
	}

	header := http.Header{}
	header.Set("Content-Type", "application/json")
	header.Set("Accept", "application/json")
	header.Set("X-Webhook-Event", h.event)
	if inbound != nil {
		for _, name := range h.forward {
			if value := inbound.Header.Get(name); value != "" {
				header.Set(name, value)
			}
		}
	}

	resp, err := h.client.Do(ctx, outbound.Request{
		Method:       h.method,
		URL:          h.url,
		Body:         body,
		Header:       header,
		IgnoreStatus: h.response.Ignore,
	})
	if err != nil {
		if statusErr, ok := outbound.AsStatusError(err); ok {
			result.StatusCode = statusErr.StatusCode
		}
		result.Err = err
		return result
	}

	result.StatusCode = resp.StatusCode
	if h.response.Parse {
		if !json.Valid(resp.Body) {
			result.Err = fmt.Errorf("webhook: response body is not valid json")
			return result
		}
		result.Body = json.RawMessage(resp.Body)
	}

	logger.Debug("webhook delivered",
		zap.String("event", h.event),
		zap.String("hook", h.id),
		zap.Int("status_code", resp.StatusCode),
	)

	return result
}

// actorFrom builds the envelope's actor block from the request context. A
// context with no actor (a background dispatch) yields a zero actor rather than
// an error: the identity is informational, not a delivery precondition.
func actorFrom(ctx context.Context) Actor {
	actor := rbac.FromContext(ctx)
	if actor == nil {
		return Actor{}
	}
	return Actor{
		Type:           string(actor.Type),
		ID:             actor.ID,
		OrganizationID: uuidString(actor.OrganizationID),
		ProjectID:      uuidString(actor.ProjectID),
	}
}

// uuidString renders a UUID for the envelope, mapping the nil UUID to an empty
// string so a template can test it for truthiness.
func uuidString(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}
