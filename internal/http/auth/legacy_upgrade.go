package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"github.com/lunogram/platform/internal/node/metrics"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

// loginCallbackPrefix is the path an explicit login posts to. The upgrade skips
// it: a login mints its own session, and running both would leave the browser
// with two rows for one sign-in.
const loginCallbackPrefix = "/api/auth/login/"

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
	// Collapses a burst of requests carrying the SAME legacy cookie into one
	// upgrade. A console page load fires many requests in parallel and none of
	// them has the new cookie yet, so without this they would all race to prove
	// the same credential. Reuse (see [Exchanger.Upgrade]) is what bounds the
	// session count; this is what stops the burst doing the work N times.
	var upgrades singleflight.Group

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if verifier == nil || exchanger == nil || signer == nil {
				next.ServeHTTP(w, r)
				return
			}

			if strings.HasPrefix(r.URL.Path, loginCallbackPrefix) {
				next.ServeHTTP(w, r)
				return
			}

			// A caller that already holds a console session needs nothing from
			// us, and must not pay for an upstream verification on every request.
			if _, ok := consoleCookie(r); ok {
				next.ServeHTTP(w, r)
				return
			}

			legacy, err := r.Cookie(LegacySessionCookie)
			if err != nil || legacy.Value == "" {
				next.ServeHTTP(w, r)
				return
			}

			result, err, _ := upgrades.Do(credentialKey(legacy.Value), func() (any, error) {
				return upgrade(r, verifier, exchanger)
			})
			if err != nil {
				// Not a credential we can upgrade, or the upgrade failed. Pass it
				// through and let the normal authentication path reject it with a
				// 401, so this middleware never becomes a second place that
				// decides whether a request is authorised.
				metrics.AuthLegacySessionUpgradeTotal.WithLabelValues(upgradeOutcome(err)).Inc()
				if !errors.Is(err, ErrUnauthorized) {
					logger.Warn("failed to upgrade a legacy session cookie", zap.Error(err))
				}
				next.ServeHTTP(w, r)
				return
			}

			upgraded := result.(*ExchangeResult)
			SetConsoleSessionCookie(w, r, upgraded.Token, upgraded.ExpiresAt)

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
			r.AddCookie(&http.Cookie{Name: consoleCookieName(r), Value: upgraded.Token})

			metrics.AuthLegacySessionUpgradeTotal.WithLabelValues("upgraded").Inc()
			next.ServeHTTP(w, r)
		})
	}
}

// upgrade verifies the legacy credential and exchanges it. It runs under
// singleflight, so it must not touch the ResponseWriter -- only one caller of a
// collapsed burst would own it. Each caller sets its own cookie from the shared
// result.
func upgrade(r *http.Request, verifier Verifier, exchanger *Exchanger) (*ExchangeResult, error) {
	// The leader's cancellation is deliberately dropped. Followers share its
	// result, so a browser abandoning the one request that happened to lead
	// would otherwise fail every request queued behind it -- during exactly the
	// window where the whole point is that nobody gets logged out.
	ctx := context.WithoutCancel(r.Context())

	identity, err := verifier.Verify(ctx, r)
	if err != nil {
		return nil, ErrUnauthorized
	}

	return exchanger.Upgrade(ctx, r, identity)
}

// credentialKey is the singleflight key: a digest of the presented credential,
// so the token itself never becomes a map key that could surface in a dump or a
// profile.
func credentialKey(token string) string {
	sum := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// upgradeOutcome labels the metric so a credential we simply cannot verify is
// distinguishable from an upgrade that broke.
func upgradeOutcome(err error) string {
	if errors.Is(err, ErrUnauthorized) {
		return "rejected"
	}
	return "failed"
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
