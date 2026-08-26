package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

// WithAdminSession authenticates a console request. It is the single hot-path
// verifier for the admin surface: one algorithm, one issuer, one audience, one
// keyring, and a `sub` that is always a Lunogram admin UUID.
//
// The chain is deliberately short and entirely fail-closed:
//
//  1. verify the token (ES256 pinned, issuer/audience/expiry required, `kid`
//     dispatched -- see [ConsoleSigner.Verify]),
//  2. reject a cookie-borne unsafe request whose Origin is not ours,
//  3. resolve the named session and reject it if revoked, idle-expired or past
//     its absolute lifetime,
//  4. reject a token whose `sub` disagrees with the session's admin,
//  5. resolve the active organization against CURRENT membership.
//
// Step 5 is the reason a demoted or removed admin loses access immediately
// rather than at token expiry, and it is why no role, organization or email is
// carried in the token at all.
func WithAdminSession(mgmt *management.State, signer *ConsoleSigner, publicBaseURL string, logger *zap.Logger) Handler {
	return func(ctx context.Context, tokenString string) (context.Context, error) {
		if tokenString == "" || signer == nil {
			return ctx, ErrUnauthorized
		}

		claims, err := signer.Verify(tokenString)
		if err != nil {
			return ctx, ErrUnauthorized
		}

		if err := enforceConsoleOrigin(ctx, publicBaseURL); err != nil {
			return ctx, err
		}

		session, err := mgmt.GetAdminSession(ctx, claims.SessionID)
		if err != nil {
			// A lookup failure is never a pass. Returning ErrUnauthorized here
			// (rather than the raw error) also stops the response from
			// distinguishing "no such session" from "database is unwell", which
			// would otherwise let an attacker probe for live session ids.
			return ctx, ErrUnauthorized
		}

		if !session.Active(time.Now()) {
			return ctx, ErrUnauthorized
		}

		// A token may not name a session belonging to someone else. The two are
		// signed and stored independently, so this catches both a mint bug and a
		// token whose subject was tampered with under a still-valid signature
		// (which cannot happen, but costs one comparison to rule out).
		if session.AdminID != claims.AdminID {
			return ctx, ErrUnauthorized
		}

		admin, err := mgmt.GetAdmin(ctx, session.AdminID)
		if err != nil {
			return ctx, ErrUnauthorized
		}

		orgID, err := resolveActiveOrganization(ctx, mgmt, admin)
		if err != nil {
			// A real failure resolving the active organization (e.g. a transient
			// DB error) must NOT fail open onto the home org — that would bypass
			// the revoked-membership check on a hot security path. Surface the
			// error so the request fails instead of silently granting scope.
			return ctx, err
		}

		options := []rbac.ActorOption{rbac.WithOrganizationID(orgID)}
		if session.Impersonated {
			impersonator := ""
			if session.ImpersonatorAdminID != nil {
				impersonator = session.ImpersonatorAdminID.String()
			}
			options = append(options, rbac.WithImpersonation(impersonator))

			// Every impersonated request is recorded. The actor carries the
			// attribution too, but nothing downstream is obliged to log it, and
			// "who was actually driving this session" is precisely the question
			// an audit needs answered.
			if logger != nil {
				logger.Info("authenticated an impersonated session",
					zap.String("admin_id", session.AdminID.String()),
					zap.String("session_id", session.ID.String()),
					zap.String("impersonator_subject", stringValue(session.ImpersonatorSubject)),
					zap.String("impersonator_admin_id", impersonator),
					zap.String("method", requestMethod(ctx)),
					zap.String("path", requestPath(ctx)))
			}
		}

		actor := rbac.NewActor(rbac.ActorAdmin, admin.ID.String(), options...)
		return rbac.WithActor(ctx, actor), nil
	}
}

// enforceConsoleOrigin is CSRF hardening for cookie-borne credentials. A cookie
// is attached by the browser automatically, so an unsafe request carrying one
// may have been triggered from any page the admin happened to visit; an
// Authorization header cannot be set cross-origin without our consent and is
// therefore left alone.
//
// It fails closed on a mismatch and is a single string comparison. A request
// with no Origin header at all is permitted: browsers omit it on same-origin
// navigations, and non-browser clients (which are not subject to CSRF) never
// send it.
func enforceConsoleOrigin(ctx context.Context, publicBaseURL string) error {
	r := RequestFromContext(ctx)
	if r == nil || publicBaseURL == "" {
		return nil
	}
	if _, fromCookie := cookieCredential(r); !fromCookie {
		return nil
	}
	if safeMethod(r.Method) {
		return nil
	}
	origin := r.Header.Get("Origin")
	if origin == "" || origin == publicBaseURL {
		return nil
	}
	return ErrUnauthorized
}

// safeMethod reports whether the method is one RFC 9110 defines as safe, and so
// one that cannot on its own change server state.
func safeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return false
	}
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func requestMethod(ctx context.Context) string {
	if r := RequestFromContext(ctx); r != nil {
		return r.Method
	}
	return ""
}

func requestPath(ctx context.Context) string {
	if r := RequestFromContext(ctx); r != nil {
		return r.URL.Path
	}
	return ""
}
