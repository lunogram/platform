package auth

import (
	"net/http"

	"github.com/lunogram/platform/internal/node/metrics"
	"go.uber.org/zap"
)

// UpgradeLegacySession exchanges a provider-issued session cookie for a
// Lunogram console session, in flight.
//
// Without it this release logs out every admin who is currently signed in:
// production browsers hold a Clerk-issued `__session` cookie, and
// [WithAdminSession] -- which by design accepts only our own token -- would
// reject all of them with a 401 the moment the deploy lands.
//
// It is a plain chi middleware rather than another [Handler] because it needs
// the ResponseWriter to set the console cookie, and the OpenAPI
// AuthenticationFunc signature does not provide one. It must be mounted OUTSIDE
// the OpenAPI validator so it runs before authentication.
//
// This is transitional. It can be deleted once BOTH hold: the release has been
// live everywhere for longer than the upstream's session lifetime (so every
// legacy cookie has expired), and lunogram_auth_legacy_session_upgrade_total has
// been flat at zero across that window, with no identity rows left carrying the
// sentinel legacy issuer.
func UpgradeLegacySession(verifier Verifier, exchanger *Exchanger, signer *ConsoleSigner, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if verifier == nil || exchanger == nil || signer == nil {
				next.ServeHTTP(w, r)
				return
			}

			// A caller that already holds a console session needs nothing from
			// us, and must not pay for an upstream verification on every request.
			if _, ok := consoleCookie(r); ok {
				next.ServeHTTP(w, r)
				return
			}

			if cookie, err := r.Cookie(LegacySessionCookie); err != nil || cookie.Value == "" {
				next.ServeHTTP(w, r)
				return
			}

			identity, err := verifier.Verify(r.Context(), r)
			if err != nil {
				// Not a credential we can upgrade. Pass it through and let the
				// normal authentication path reject it with a 401, so this
				// middleware never becomes a second place that decides whether a
				// request is authorised.
				metrics.AuthLegacySessionUpgradeTotal.WithLabelValues("rejected").Inc()
				next.ServeHTTP(w, r)
				return
			}

			result, err := exchanger.Exchange(r.Context(), w, r, identity)
			if err != nil {
				metrics.AuthLegacySessionUpgradeTotal.WithLabelValues("failed").Inc()
				logger.Warn("failed to upgrade a legacy session cookie", zap.Error(err))
				next.ServeHTTP(w, r)
				return
			}

			// Rewriting the in-flight request is what makes the upgrade
			// invisible: THIS request succeeds under the new session rather than
			// 401-ing once while the browser learns the new cookie.
			//
			// The new credential is attached as a COOKIE, not as an Authorization
			// header: [GetSession] reads cookies at a higher precedence than the
			// header, and the legacy cookie is still on this request, so a header
			// would simply be ignored in favour of the credential we just
			// replaced. The console cookie names are checked first, so adding one
			// here shadows the legacy cookie for the rest of the request.
			r.AddCookie(&http.Cookie{Name: consoleCookieName(r), Value: result.Token})

			metrics.AuthLegacySessionUpgradeTotal.WithLabelValues("upgraded").Inc()
			next.ServeHTTP(w, r)
		})
	}
}

// consoleCookie returns the console session cookie, under whichever of its two
// names the browser is holding it.
func consoleCookie(r *http.Request) (string, bool) {
	for _, name := range []string{ConsoleCookieSecure, ConsoleCookieInsecure} {
		if cookie, err := r.Cookie(name); err == nil && cookie.Value != "" {
			return cookie.Value, true
		}
	}
	return "", false
}
