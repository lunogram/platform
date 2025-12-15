package http

import (
	"net/http"

	"go.uber.org/zap"
)

func Logger(l *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			l.Debug("incoming request",
				zap.String("proto", r.Proto),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path))

			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}
