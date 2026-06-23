package http

import (
	"net/http"
	"testing"

	"github.com/lunogram/platform/internal/rbac"
)

func TestRateLimitKey(t *testing.T) {
	t.Parallel()

	withActor := func(actor *rbac.Actor) *http.Request {
		r := &http.Request{RemoteAddr: "203.0.113.7:5555", Header: http.Header{}}
		if actor == nil {
			return r
		}
		return r.WithContext(rbac.WithActor(r.Context(), actor))
	}

	t.Run("unauthenticated requests key on the client IP", func(t *testing.T) {
		got := rateLimitKey(withActor(nil), 0)
		if got != "ip:203.0.113.7" {
			t.Errorf("rateLimitKey = %q, want %q", got, "ip:203.0.113.7")
		}
	})

	t.Run("a backend key (no subject) keys on the auth method id", func(t *testing.T) {
		actor := rbac.NewActor(rbac.ActorAPIKey, "method-1")
		got := rateLimitKey(withActor(actor), 0)
		if got != "key:method-1" {
			t.Errorf("rateLimitKey = %q, want %q", got, "key:method-1")
		}
	})

	t.Run("a verified subject partitions the bucket by subject", func(t *testing.T) {
		actor := rbac.NewActor(rbac.ActorEndUser, "method-1", rbac.WithSubject("user-9", "https://idp.test"))
		got := rateLimitKey(withActor(actor), 0)
		if got != "key:method-1:user-9" {
			t.Errorf("rateLimitKey = %q, want %q", got, "key:method-1:user-9")
		}
	})

	t.Run("two subjects on one method get distinct buckets", func(t *testing.T) {
		a := rbac.NewActor(rbac.ActorEndUser, "method-1", rbac.WithSubject("user-a", "src"))
		b := rbac.NewActor(rbac.ActorEndUser, "method-1", rbac.WithSubject("user-b", "src"))
		if rateLimitKey(withActor(a), 0) == rateLimitKey(withActor(b), 0) {
			t.Error("distinct subjects on the same method must not share a rate-limit bucket")
		}
	})
}

func TestClientIP(t *testing.T) {
	t.Parallel()

	req := func(xff string) *http.Request {
		r := &http.Request{RemoteAddr: "203.0.113.7:5555", Header: http.Header{}}
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		return r
	}

	tests := []struct {
		name string
		xff  string
		hops int
		want string
	}{
		{"no trusted proxy ignores spoofed XFF", "1.2.3.4", 0, "203.0.113.7"},
		{"single trusted proxy takes client hop", "9.9.9.9, 1.2.3.4", 1, "1.2.3.4"},
		{"one proxy, single XFF entry is the client", "1.2.3.4", 1, "1.2.3.4"},
		{"two trusted proxies", "5.5.5.5, 9.9.9.9, 1.2.3.4", 2, "9.9.9.9"},
		{"more hops than chain clamps to left-most", "1.2.3.4", 5, "1.2.3.4"},
		{"no XFF falls back to remote", "", 1, "203.0.113.7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clientIP(req(tt.xff), tt.hops); got != tt.want {
				t.Errorf("clientIP(hops=%d, xff=%q) = %q, want %q", tt.hops, tt.xff, got, tt.want)
			}
		})
	}
}
