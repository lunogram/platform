package metrics

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/cloudproud/graceful"
	"go.uber.org/zap"
)

// Serve exposes the Prometheus registry on its own listener until ctx closes.
//
// Metrics deliberately do not share the public HTTP server. That server mounts
// the console on a catch-all route and sits behind the public ingress, so
// anything registered there is reachable from the internet; a separate port
// keeps the registry inside the cluster. It also skips the OpenTelemetry
// wrapper the API server uses, which would otherwise open a span for every
// scrape.
func Serve(ctx graceful.Context, logger *zap.Logger, address string) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		// Losing metrics must not take the node down with it -- the node is
		// still perfectly able to serve traffic without them.
		logger.Error("failed to listen for metrics, continuing without them",
			zap.String("address", address), zap.Error(err))
		return
	}

	serve(ctx, logger, listener)
}

// serve runs the metrics server on an already-open listener. Split out so a
// test can hand it an ephemeral port and exercise the routes and the shutdown
// this package actually installs, rather than a mux assembled to look like it.
func serve(ctx graceful.Context, logger *zap.Logger, listener net.Listener) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", NewHandler())
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       5 * time.Second,
		// Generous relative to the API server: a registry with a label per
		// project takes a while to render, and a scrape cut off mid-write is
		// reported as a failed target rather than a slow one.
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// Registered on the calling goroutine: a closer added from one of its own
	// races the shutdown it is meant to handle, and a scrape endpoint that
	// misses the signal holds the process open.
	ctx.Closer(func() {
		logger.Info("received close signal, metrics server shutting down")
		if err := server.Shutdown(ctx); err != nil && !errors.Is(ctx.Err(), context.Canceled) {
			logger.Debug("failed to shutdown metrics server", zap.Error(err))
		}
	})

	logger.Info("serving prometheus metrics", zap.Stringer("address", listener.Addr()))

	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("metrics server stopped", zap.Error(err))
	}
}
