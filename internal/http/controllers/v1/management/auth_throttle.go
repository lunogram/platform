package v1

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	lunohttp "github.com/lunogram/platform/internal/http"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/ratelimit"
)

// Throttling budgets for the credential endpoints.
//
// These sit under the ordinary per-request rate limit and exist for a different
// reason: that one protects the server, these protect one account. The numbers
// are chosen so a person who has genuinely forgotten which password they used
// is never locked out, while an attacker gets nowhere near the thousands of
// guesses an online attack needs.
const (
	// loginFailureLimit is how many failed sign-ins an account absorbs before it
	// stops answering. Failures only: a correct password never spends budget, so
	// somebody logging in on five devices is unaffected.
	loginFailureLimit  = 10
	loginFailureWindow = 15 * time.Minute

	// loginIPLimit bounds failures from one address across all accounts, which
	// is what a password-spray looks like: one guess against each of a thousand
	// accounts never trips any single account's budget.
	loginIPLimit  = 60
	loginIPWindow = 15 * time.Minute

	// resetLimit bounds password-reset requests per account. Each one sends
	// mail, so an unbounded endpoint is a way to bury somebody's inbox using
	// our domain's reputation.
	resetLimit  = 5
	resetWindow = time.Hour

	// resetIPLimit and registerIPLimit bound how much mail one source can cause
	// to be sent to third parties. Without them the endpoints are a spam cannon
	// with our SPF record attached.
	resetIPLimit     = 20
	resetIPWindow    = time.Hour
	registerIPLimit  = 10
	registerIPWindow = time.Hour
)

// throttle applies per-account and per-source budgets to the credential
// endpoints.
//
// Every budget is keyed on a digest rather than the value itself: these keys
// live in a shared Redis that is not the place to keep a list of the addresses
// that hold accounts here.
type throttle struct {
	limiter          *ratelimit.Limiter
	trustedProxyHops int
}

func newThrottle(limiter *ratelimit.Limiter, trustedProxyHops int) *throttle {
	return &throttle{limiter: limiter, trustedProxyHops: trustedProxyHops}
}

// budget is one throttling rule: a key prefix, a ceiling and a window.
type budget struct {
	prefix string
	limit  int
	window time.Duration
}

var (
	loginAccountBudget = budget{prefix: "auth:login:account:", limit: loginFailureLimit, window: loginFailureWindow}
	loginSourceBudget  = budget{prefix: "auth:login:ip:", limit: loginIPLimit, window: loginIPWindow}
	resetAccountBudget = budget{prefix: "auth:reset:account:", limit: resetLimit, window: resetWindow}
	resetSourceBudget  = budget{prefix: "auth:reset:ip:", limit: resetIPLimit, window: resetIPWindow}
	registerSourceUse  = budget{prefix: "auth:register:ip:", limit: registerIPLimit, window: registerIPWindow}
)

// exceeded reports whether any of the given budgets is already spent, and how
// long the caller should wait. It records nothing.
func (t *throttle) exceeded(ctx context.Context, keys map[budget]string) (bool, time.Duration) {
	var longest time.Duration
	var tripped bool

	for rule, key := range keys {
		if key == "" {
			continue
		}
		spent, retryAfter := t.limiter.Exceeded(ctx, rule.prefix+key, rule.limit, rule.window)
		if spent {
			tripped = true
			longest = max(longest, retryAfter)
		}
	}

	return tripped, longest
}

// spend records one use against each budget.
func (t *throttle) spend(ctx context.Context, keys map[budget]string) {
	for rule, key := range keys {
		if key == "" {
			continue
		}
		_, _, _ = t.limiter.Allow(ctx, rule.prefix+key, rule.limit, rule.window)
	}
}

// accountKey identifies an account for throttling. It is derived from the
// submitted address, not from an account we found, so an address with no
// account is throttled exactly like one that has an account -- otherwise the
// throttle itself becomes the enumeration oracle the endpoints are careful not
// to be.
func accountKey(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(email))
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}

func (t *throttle) sourceKey(r *http.Request) string {
	sum := sha256.Sum256([]byte(lunohttp.ClientIP(r, t.trustedProxyHops)))
	return base64.RawURLEncoding.EncodeToString(sum[:16])
}

// writeTooManyRequests answers a tripped budget. The body carries no hint about
// which budget tripped: "this account is locked" and "this address is sending
// too much" are different facts about different things, and only one of them is
// the caller's business.
func writeTooManyRequests(w http.ResponseWriter, retryAfter time.Duration) {
	if retryAfter > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
	}
	oapi.WriteProblem(w, problem.ErrTooManyRequests(problem.Describe("too many attempts, please try again later")))
}
