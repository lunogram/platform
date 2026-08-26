//go:build enterprise

package v1

import (
	"bufio"
	"context"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/http"
	"github.com/lunogram/platform/internal/http/auth"
	mgmtoapi "github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/rbac"
	"go.uber.org/zap"
)

const testSessionToken = "valid-admin-session"

var (
	testAdminID = uuid.New()
	testOrgID   = uuid.New()
)

// testAdminSession stands in for auth.WithJWT: it accepts exactly one session
// token and resolves it to an admin actor, without needing a signing key or the
// management database.
func testAdminSession() auth.Handler {
	return func(ctx context.Context, token string) (context.Context, error) {
		if token != testSessionToken {
			return ctx, auth.ErrUnauthorized
		}

		actor := rbac.NewActor(rbac.ActorAdmin, testAdminID.String(), rbac.WithOrganizationID(testOrgID))
		return rbac.WithActor(ctx, actor), nil
	}
}

// mountTestProxies wires the enterprise proxy routes exactly as NewServer does,
// with both upstreams pointed at the given test server.
func mountTestProxies(upstreamURL string) chi.Router {
	router := chi.NewRouter()
	MountProxyRoutes(
		zap.NewNop(),
		router,
		config.Enterprise{
			Proxy: config.EnterpriseProxy{
				BackofficeURL: upstreamURL,
				CourierURL:    upstreamURL,
			},
		},
		auth.Require(mgmtoapi.WriteProblem, testAdminSession()),
	)
	return router
}

func sessionCookie(r *nethttp.Request) {
	r.AddCookie(&nethttp.Cookie{Name: "__session", Value: testSessionToken})
}

// TestProxyRoutesRejectUnauthenticatedRequests covers the reason these routes
// carry authentication at all. They are registered on the root router, outside
// the OpenAPI validator that authenticates every other management endpoint, and
// neither upstream checks a credential of its own — so without the middleware a
// request with no credential at all reached the backoffice and courier APIs over
// the public ingress.
func TestProxyRoutesRejectUnauthenticatedRequests(t *testing.T) {
	reached := false
	upstream := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		reached = true
		w.WriteHeader(nethttp.StatusOK)
	}))
	defer upstream.Close()

	router := mountTestProxies(upstream.URL)

	credentials := map[string]func(*nethttp.Request){
		"no credential": func(*nethttp.Request) {},
		"unknown bearer token": func(r *nethttp.Request) {
			r.Header.Set("Authorization", "Bearer not-a-session")
		},
		"unknown session cookie": func(r *nethttp.Request) {
			r.AddCookie(&nethttp.Cookie{Name: "__session", Value: "not-a-session"})
		},
	}

	paths := []string{"/backoffice/v1/conversations", "/courier/v1/domains"}

	for _, path := range paths {
		for name, credential := range credentials {
			t.Run(path+" with "+name, func(t *testing.T) {
				reached = false

				req := httptest.NewRequest(nethttp.MethodGet, path, nil)
				credential(req)
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)

				if rec.Code != nethttp.StatusUnauthorized {
					t.Errorf("status = %d, want %d", rec.Code, nethttp.StatusUnauthorized)
				}

				if reached {
					t.Error("request reached the upstream service without a valid credential")
				}
			})
		}
	}
}

func TestProxyRoutesForwardAuthenticatedRequests(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		gotPath = r.URL.Path
		io.WriteString(w, "upstream response") //nolint:errcheck
	}))
	defer upstream.Close()

	router := mountTestProxies(upstream.URL)

	routes := map[string]string{
		"/backoffice/v1/conversations": "/v1/conversations",
		"/courier/v1/domains":          "/v1/domains",
	}

	for path, upstreamPath := range routes {
		t.Run(path, func(t *testing.T) {
			gotPath = ""

			req := httptest.NewRequest(nethttp.MethodGet, path, nil)
			sessionCookie(req)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)

			if rec.Code != nethttp.StatusOK {
				t.Fatalf("status = %d, want %d", rec.Code, nethttp.StatusOK)
			}

			if body := rec.Body.String(); body != "upstream response" {
				t.Errorf("body = %q, want the upstream response", body)
			}

			if gotPath != upstreamPath {
				t.Errorf("upstream path = %q, want %q", gotPath, upstreamPath)
			}
		})
	}
}

// TestProxyForwardsAuthenticatedIdentityOverClientHeaders checks that the
// identity an upstream service sees is the one this server authenticated, not
// one the caller supplied. The Connection header is part of the attempt: a
// reverse proxy deletes the headers it names, which would otherwise let a caller
// suppress its own identity on the way upstream.
func TestProxyForwardsAuthenticatedIdentityOverClientHeaders(t *testing.T) {
	var gotActor, gotOrg string
	upstream := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		gotActor = r.Header.Get(http.HeaderActorID)
		gotOrg = r.Header.Get(http.HeaderOrganizationID)
	}))
	defer upstream.Close()

	router := mountTestProxies(upstream.URL)

	req := httptest.NewRequest(nethttp.MethodGet, "/backoffice/v1/conversations", nil)
	sessionCookie(req)
	req.Header.Set(http.HeaderActorID, uuid.New().String())
	req.Header.Set(http.HeaderOrganizationID, uuid.New().String())
	req.Header.Set("Connection", http.HeaderActorID+", "+http.HeaderOrganizationID)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if gotActor != testAdminID.String() {
		t.Errorf("upstream actor id = %q, want the authenticated admin %q", gotActor, testAdminID)
	}

	if gotOrg != testOrgID.String() {
		t.Errorf("upstream organization id = %q, want the authenticated organization %q", gotOrg, testOrgID)
	}
}

// TestProxyPreservesStreaming guards the AI builder, which reads its replies as
// a server-sent event stream: nothing in the authentication chain may wrap the
// response writer in a way that costs it its http.Flusher, or the console would
// only render a reply once the upstream finished writing it.
func TestProxyPreservesStreaming(t *testing.T) {
	read := make(chan struct{})
	upstream := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		w.Header().Set("Content-Type", "text/event-stream")

		flusher, ok := w.(nethttp.Flusher)
		if !ok {
			t.Error("upstream response writer is not an http.Flusher")
			return
		}

		io.WriteString(w, "data: first\n\n") //nolint:errcheck
		flusher.Flush()

		// The handler only writes its second chunk once the client has read the
		// first, so reading it proves the first was delivered mid-response
		// rather than buffered until the handler returned.
		select {
		case <-read:
		case <-time.After(5 * time.Second):
			t.Error("timed out waiting for the client to read the flushed chunk")
			return
		}

		io.WriteString(w, "data: second\n\n") //nolint:errcheck
		flusher.Flush()
	}))
	defer upstream.Close()

	proxy := httptest.NewServer(mountTestProxies(upstream.URL))
	defer proxy.Close()

	req, err := nethttp.NewRequest(nethttp.MethodGet, proxy.URL+"/backoffice/v1/conversations/1/messages", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	sessionCookie(req)

	resp, err := nethttp.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)

	first, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("failed to read the first streamed chunk: %v", err)
	}
	if got := strings.TrimSpace(first); got != "data: first" {
		t.Fatalf("first chunk = %q, want %q", got, "data: first")
	}

	close(read)

	rest, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("failed to read the remainder of the stream: %v", err)
	}
	if !strings.Contains(string(rest), "data: second") {
		t.Errorf("remainder = %q, want it to contain %q", string(rest), "data: second")
	}
}
