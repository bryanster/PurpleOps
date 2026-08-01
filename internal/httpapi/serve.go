package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/bryanster/purpleops/internal/config"
)

// Transport-level timeouts. They are constants rather than configuration
// because they are properties of the protocol, not of the deployment: an
// operator tuning how long a request may take wants PURPLEOPS_REQUEST_TIMEOUT.
const (
	// readHeaderTimeout bounds how long a connection may take to send its
	// request line and headers. Without it a handful of sockets dribbling one
	// byte at a time hold the server open indefinitely (and gosec G112 says so).
	readHeaderTimeout = 10 * time.Second

	// idleTimeout closes a kept-alive connection that has gone quiet, so a
	// browser tab left open overnight does not hold a file descriptor.
	idleTimeout = 2 * time.Minute
)

// Deliberately no ReadTimeout or WriteTimeout. ReadTimeout would cap how long
// an evidence upload may take (M3), and WriteTimeout would cap how long an SSE
// stream may stay open (M4) — both would fail as a truncated response rather
// than as anything a user could act on. The per-request deadline covers the
// case they are usually reached for, and it fails through the one error path.

// ListenAndServe binds cfg.Addr and serves handler until ctx is cancelled,
// then shuts down gracefully: no new connections, in-flight requests finish
// within cfg.ShutdownTimeout, and anything still running when that expires is
// cut off so the process can exit.
//
// It returns nil for a clean shutdown, including one that had to cut a request
// off — that is a warning in the log, not a failed run, and an orchestrator
// should not see a crash where it asked for a stop.
func ListenAndServe(ctx context.Context, cfg config.Server, handler http.Handler, log *slog.Logger) error {
	listener, err := listen(ctx, cfg.Addr)
	if err != nil {
		return err
	}
	return serve(ctx, cfg, listener, handler, log)
}

// listen binds the address. It is separate from [serve] so that a test can bind
// port 0 and find out which port it got.
func listen(ctx context.Context, addr string) (net.Listener, error) {
	// A ListenConfig rather than net.Listen, so that a signal arriving while
	// the address is still being resolved is honoured.
	var config net.ListenConfig
	listener, err := config.Listen(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("httpapi: listen on %q: %w", addr, err)
	}
	return listener, nil
}

// serve runs the server on an already-bound listener.
func serve(ctx context.Context, cfg config.Server, listener net.Listener, handler http.Handler, log *slog.Logger) error {
	if log == nil {
		log = slog.Default()
	}

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		// net/http's own complaints — a malformed request line, a TLS
		// handshake failure — otherwise go to the standard logger and miss the
		// configured format entirely.
		ErrorLog: slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	// Buffered: if serve returns before reading it — it does not, but a future
	// edit might — the goroutine still finishes rather than leaking.
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()

	log.InfoContext(ctx, "listening", slog.String("addr", listener.Addr().String()))

	select {
	case err := <-served:
		// Serve gave up on its own: the listener failed. It cannot be
		// ErrServerClosed here, because nothing has asked it to stop yet.
		return fmt.Errorf("httpapi: serve: %w", err)
	case <-ctx.Done():
	}

	log.Info("shutting down", slog.Duration("grace", cfg.ShutdownTimeout))

	// Not ctx: it is already cancelled, and Shutdown's context is the deadline
	// for draining rather than a reason to start.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		// A request outlived the grace period. Close cuts the connections it is
		// holding, because a process that will not exit is worse than a request
		// that does not finish — an orchestrator kills it, and the store never
		// gets closed cleanly.
		log.Warn("graceful shutdown did not finish in time; closing connections",
			slog.Duration("grace", cfg.ShutdownTimeout),
			slog.String("error", err.Error()))
		if err := server.Close(); err != nil {
			return fmt.Errorf("httpapi: close the server: %w", err)
		}
	}

	// Shutdown and Close both make Serve return ErrServerClosed. Anything else
	// is a real failure that happened on the way out.
	if err := <-served; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("httpapi: serve: %w", err)
	}
	return nil
}
