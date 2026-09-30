package main

import (
	"context"
	"errors"
	"net/http"
	"time"
)

func serve(ctx context.Context, srv *http.Server, handler http.Handler) error {
	result := make(chan error, 1)
	go func() { result <- srv.ListenAndServe() }()
	var listenErr error
	select {
	case listenErr = <-result:
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	lifecycle, ok := handler.(interface {
		BeginShutdown()
		Shutdown(context.Context) error
	})
	if ok {
		lifecycle.BeginShutdown()
	}
	httpErr := srv.Shutdown(shutdownCtx)
	if httpErr != nil {
		_ = srv.Close()
	}
	var workerErr error
	if ok {
		workerErr = lifecycle.Shutdown(shutdownCtx)
	}
	if errors.Is(listenErr, http.ErrServerClosed) {
		listenErr = nil
	}
	return errors.Join(listenErr, httpErr, workerErr)
}
