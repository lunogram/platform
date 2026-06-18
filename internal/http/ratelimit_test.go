package http

import (
	"net/http"
	"testing"
)

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
