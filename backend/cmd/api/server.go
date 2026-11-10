package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// shutdownTimeout: systemd er TimeoutStopSec (30 second, 13.4) er kom.
const shutdownTimeout = 25 * time.Second

// newServer: 4.2 er timeout. Upload er route nijer read ar write deadline barabe (2.2, http.ResponseController).
func newServer(h http.Handler) *http.Server {
	return &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

// serve: ctx bondho hole (SIGTERM, systemd stop) notun connection bondho, cholche emon request shesh howar
// shomoy dey (graceful shutdown, 2.2).
func serve(ctx context.Context, srv *http.Server, ln net.Listener, log *slog.Logger) error {
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	log.Info("shutdown shuru")
	sctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	log.Info("shutdown shesh")
	return nil
}
