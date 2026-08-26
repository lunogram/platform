package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/rbac"
)

func writeProblem(w http.ResponseWriter, err error) {
	title, detail := problem.GetDescription(err)
	json.Write(w, problem.GetStatus(err), map[string]string{"title": title, "detail": detail})
}

func acceptToken(token string, actor *rbac.Actor) auth.Handler {
	return func(ctx context.Context, value string) (context.Context, error) {
		if value != token {
			return ctx, auth.ErrUnauthorized
		}
		return rbac.WithActor(ctx, actor), nil
	}
}

func failing(err error) auth.Handler {
	return func(ctx context.Context, _ string) (context.Context, error) {
		return ctx, err
	}
}

func TestRequireRejectsMissingAndUnknownCredentials(t *testing.T) {
	middleware := auth.Require(writeProblem, acceptToken("token", rbac.NewActor(rbac.ActorAdmin, "admin")))

	for name, authorization := range map[string]string{
		"missing credential": "",
		"unknown credential": "Bearer nope",
	} {
		t.Run(name, func(t *testing.T) {
			served := false
			handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				served = true
			}))

			req := httptest.NewRequest(http.MethodGet, "/anything", nil)
			if authorization != "" {
				req.Header.Set("Authorization", authorization)
			}

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}

			if served {
				t.Error("the wrapped handler ran for a request that was not authenticated")
			}
		})
	}
}

func TestRequirePassesActorDownstream(t *testing.T) {
	orgID := uuid.New()
	actor := rbac.NewActor(rbac.ActorAdmin, "admin", rbac.WithOrganizationID(orgID))
	middleware := auth.Require(writeProblem, acceptToken("token", actor))

	var got *rbac.Actor
	handler := middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = rbac.FromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	req.AddCookie(&http.Cookie{Name: "__session", Value: "token"})

	handler.ServeHTTP(httptest.NewRecorder(), req)

	if got == nil {
		t.Fatal("no actor reached the wrapped handler")
	}

	if got.ID != actor.ID || got.OrganizationID != orgID {
		t.Errorf("actor = %+v, want %+v", got, actor)
	}
}

// TestRequireFailsClosedOnHandlerError covers the difference between a rejected
// credential and a credential that could not be evaluated: a database error
// while resolving the actor must abort the request rather than be read as "this
// handler does not apply" and fall through to a weaker one.
func TestRequireFailsClosedOnHandlerError(t *testing.T) {
	fallback := false
	middleware := auth.Require(
		writeProblem,
		failing(errors.New("database unavailable")),
		func(ctx context.Context, _ string) (context.Context, error) {
			fallback = true
			return rbac.WithActor(ctx, rbac.NewActor(rbac.ActorAdmin, "admin")), nil
		},
	)

	served := false
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		served = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/anything", nil)
	req.AddCookie(&http.Cookie{Name: "__session", Value: "token"})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	if fallback {
		t.Error("a handler that errored fell through to the next credential")
	}

	if served {
		t.Error("the wrapped handler ran for a request that was not authenticated")
	}

	if body := rec.Body.String(); strings.Contains(body, "database unavailable") {
		t.Errorf("response body leaks the internal error: %q", body)
	}
}
