package outbound

import (
	"fmt"
	"time"

	"github.com/lunogram/platform/internal/ssrf"
)

// DefaultMaxResponseBytes bounds how much of a response body is read from an
// operator-configured endpoint. Bodies are held in memory and, for the template
// gallery, rendered in the console, so an unbounded read is a denial-of-service
// vector handed to whoever controls the configured URL.
const DefaultMaxResponseBytes int64 = 1 << 20

// Network is the per-destination SSRF relaxation, expressed in configuration.
// The zero value is the strict policy; see [ssrf.Policy].
type Network struct {
	AllowPrivate bool `yaml:"allow_private"`
	AllowHTTP    bool `yaml:"allow_http"`
}

// Policy converts the configured relaxations into the guard policy.
func (n Network) Policy() ssrf.Policy {
	return ssrf.Policy{AllowPrivate: n.AllowPrivate, AllowHTTP: n.AllowHTTP}
}

// Relaxed reports whether any guard has been dropped. Callers use this to warn
// at config-load time, so a relaxation is never invisible in the logs.
func (n Network) Relaxed() bool {
	return n.AllowPrivate || n.AllowHTTP
}

// Retry bounds how hard a single request is retried.
//
// The three limits nest: Timeout bounds one attempt, MaxElapsedTime bounds the
// whole attempt sequence for one request, and (in the webhook engine) a
// per-dispatch deadline bounds every request fired for one event. Without
// MaxElapsedTime, MaxAttempts x Timeout is the worst case, which is how a 30s
// timeout and four attempts becomes two minutes of a human waiting on an API
// call — so MaxElapsedTime is defaulted rather than left unset.
type Retry struct {
	MaxAttempts     int           `yaml:"max_attempts"`
	InitialInterval time.Duration `yaml:"initial_interval"`
	MaxInterval     time.Duration `yaml:"max_interval"`
	MaxElapsedTime  time.Duration `yaml:"max_elapsed_time"`
}

// DefaultRetry is applied where configuration omits a retry block entirely.
func DefaultRetry() Retry {
	return Retry{
		MaxAttempts:     3,
		InitialInterval: 250 * time.Millisecond,
		MaxInterval:     5 * time.Second,
	}
}

// WithDefaults fills unset fields from base, then derives MaxElapsedTime from
// the attempt budget when it is still unset so the sequence is always bounded.
func (r Retry) WithDefaults(base Retry, timeout time.Duration) Retry {
	if r.MaxAttempts == 0 {
		r.MaxAttempts = base.MaxAttempts
	}
	if r.InitialInterval == 0 {
		r.InitialInterval = base.InitialInterval
	}
	if r.MaxInterval == 0 {
		r.MaxInterval = base.MaxInterval
	}
	if r.MaxElapsedTime == 0 {
		r.MaxElapsedTime = base.MaxElapsedTime
	}
	if r.MaxElapsedTime == 0 && timeout > 0 && r.MaxAttempts > 0 {
		r.MaxElapsedTime = time.Duration(r.MaxAttempts) * timeout
	}
	return r
}

// Validate rejects retry settings that cannot describe a terminating sequence.
func (r Retry) Validate() error {
	if r.MaxAttempts < 1 {
		return fmt.Errorf("retry: max_attempts must be at least 1")
	}
	if r.InitialInterval < 0 || r.MaxInterval < 0 || r.MaxElapsedTime < 0 {
		return fmt.Errorf("retry: intervals must not be negative")
	}
	if r.MaxInterval > 0 && r.InitialInterval > r.MaxInterval {
		return fmt.Errorf("retry: initial_interval must not exceed max_interval")
	}
	return nil
}
