package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

func NewServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}

// RunServer serves HTTP until the context is canceled or the listener fails.
// Cancellation drains active requests before returning.
func RunServer(ctx context.Context, server *http.Server, logger *slog.Logger) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen HTTP: %w", err)
	}
	return serve(ctx, server, listener, logger)
}

func serve(ctx context.Context, server *http.Server, listener net.Listener, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Serve(listener)
	}()
	logger.Info("HTTP server started", "addr", listener.Addr().String())

	var serveErr error
	select {
	case <-ctx.Done():
		logger.Info("HTTP server shutdown requested")
	case serveErr = <-serverErr:
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		if closeErr := server.Close(); closeErr != nil {
			return fmt.Errorf("force shutdown HTTP server: %w", closeErr)
		}
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}
	if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
		return fmt.Errorf("serve HTTP: %w", serveErr)
	}

	logger.Info("HTTP server stopped")
	return nil
}
