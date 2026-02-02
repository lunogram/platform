package http

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"regexp"
	"time"

	"github.com/cloudproud/graceful"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/nyaruka/phonenumbers"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var emailRegex = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9-]+(?:\\.[a-zA-Z0-9-]+)*$")

func init() {
	openapi3.DefineStringFormatValidator("email", openapi3.NewCallbackValidator(func(value string) error {
		if !emailRegex.MatchString(value) {
			return fmt.Errorf("invalid email format")
		}
		return nil
	}))

	openapi3.DefineStringFormatValidator("phone", openapi3.NewCallbackValidator(func(value string) error {
		num, err := phonenumbers.Parse(value, "")
		if err != nil {
			return err
		}

		if !phonenumbers.IsValidNumber(num) {
			return errors.New("phone number must be in E.164 format (e.g., +14155552671)")
		}

		if phonenumbers.Format(num, phonenumbers.E164) != value {
			return errors.New("phone number must be in E.164 format (e.g., +14155552671)")
		}

		return nil
	}))
}

type ResponseWriter = http.ResponseWriter

// FileServer to serve static files eg public swagger ui
var FileServer = http.FileServer

// FS Filesystem of static files
var FS = http.FS

// Config holds the configuration for the http server.
type Config struct {
	// ReadHeaderTimeout is the maximum duration for reading the request header.
	ReadHeaderTimeout time.Duration `env:"READ_HEADER_TIMEOUT" envDefault:"5s"`
	// ReadTimeout is the maximum duration for reading the entire request, including the body.
	ReadTimeout time.Duration `env:"READ_TIMEOUT" envDefault:"5s"`
	// IdleTimeout is the maximum amount of time to wait for the next request when keep-alives are enabled.
	IdleTimeout time.Duration `env:"IDLE_TIMEOUT" envDefault:"5s"`
	// WriteTimeout is the maximum duration before timing out writes of the response.
	WriteTimeout time.Duration `env:"WRITE_TIMEOUT" envDefault:"10s"`
	// MaxHeaderBytes controls the maximum number of bytes the server will read parsing the request header's keys and values, including the request line.
	MaxHeaderBytes int `env:"MAX_HEADER_BYTES" envDefault:"1048576"`
	// CSRF holds the CSRF protection configuration.
	CSRF CSRFConfig `envPrefix:"CSRF_"`
}

// CSRFConfig holds the configuration for CSRF protection.
type CSRFConfig struct {
	// Enabled determines whether CSRF protection is enabled.
	Enabled bool `env:"ENABLED" envDefault:"true"`
	// Secret is the secret key used to generate CSRF tokens.
	// If not provided, a random key will be generated on startup.
	Secret string `env:"SECRET"`
	// TokenLength is the length of the CSRF token in bytes.
	TokenLength int `env:"TOKEN_LENGTH" envDefault:"32"`
	// CookieName is the name of the CSRF cookie.
	CookieName string `env:"COOKIE_NAME" envDefault:"csrf_token"`
	// HeaderName is the name of the CSRF header.
	HeaderName string `env:"HEADER_NAME" envDefault:"X-CSRF-Token"`
	// FieldName is the name of the CSRF form field.
	FieldName string `env:"FIELD_NAME" envDefault:"csrf_token"`
	// CookieSecure determines whether the CSRF cookie should be secure (HTTPS only).
	CookieSecure bool `env:"COOKIE_SECURE" envDefault:"true"`
	// CookieHTTPOnly determines whether the CSRF cookie should be HTTP only.
	CookieHTTPOnly bool `env:"COOKIE_HTTP_ONLY" envDefault:"true"`
	// CookieSameSite determines the SameSite attribute of the CSRF cookie.
	// Valid values: "strict", "lax", "none", "default"
	CookieSameSite string `env:"COOKIE_SAME_SITE" envDefault:"strict"`
	// TrustedOrigins is a list of trusted origins for CSRF validation.
	// If empty, only same-origin requests are allowed.
	TrustedOrigins []string `env:"TRUSTED_ORIGINS"`
}

// NewServer creates a new http *Server with the given logger. A trace handler
// will be wrapped around the provided handler.
func NewServer(logger *zap.Logger, handler http.Handler, config Config) *Server {
	stdLogger, err := zap.NewStdLogAt(logger, zapcore.ErrorLevel)
	if err != nil {
		logger.Error("failed to inject logger. fallback to std log", zap.Error(err))
		stdLogger = slog.NewLogLogger(slog.NewJSONHandler(os.Stderr, nil), slog.LevelError)
	}

	return &Server{
		logger: logger,
		Server: &http.Server{
			ReadHeaderTimeout: config.ReadHeaderTimeout,
			ReadTimeout:       config.ReadTimeout,
			IdleTimeout:       config.IdleTimeout,
			WriteTimeout:      config.WriteTimeout,
			MaxHeaderBytes:    config.MaxHeaderBytes,
			ErrorLog:          stdLogger,
			Handler:           otelhttp.NewHandler(handler, "http-server"),
		},
	}
}

// Server wraps the http.Server with a logger and custom Serve method
type Server struct {
	logger *zap.Logger
	*http.Server
}

// Serve starts the http server on the given address and listens for incoming connections.
// The server will gracefully shutdown when the provided context is closed.
func (srv *Server) Serve(ctx graceful.Context, address string) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		srv.logger.Fatal("failed to establish HTTP connection, shutting down...", zap.Error(err))
		ctx.Shutdown()
	}

	go ctx.Closer(func() {
		srv.logger.Info("received close signal, http server shutting down")
		if err = srv.Shutdown(ctx); err != nil && !errors.Is(ctx.Err(), context.Canceled) {
			srv.logger.Debug("failed to shutdown http server", zap.Error(err))
		}
	})

	srv.logger.Info("serving incoming http connections", zap.Stringer("address", listener.Addr()))

	err = srv.Server.Serve(listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		srv.logger.Fatal("failure serving http connections, http server shutting down", zap.Error(err))
		ctx.Shutdown()
	}
}
