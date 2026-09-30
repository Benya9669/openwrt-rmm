package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

type drainTestHandler struct {
	entered  chan struct{}
	release  chan struct{}
	stopping chan struct{}
	stopped  chan struct{}
}

func (h *drainTestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	close(h.entered)
	<-h.release
	w.WriteHeader(http.StatusOK)
}
func (h *drainTestHandler) BeginShutdown()                 { close(h.stopping) }
func (h *drainTestHandler) Shutdown(context.Context) error { close(h.stopped); return nil }

func TestServeDrainsInflightRequests(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	handler := &drainTestHandler{make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})}
	srv := &http.Server{Addr: address, Handler: handler}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- serve(ctx, srv, handler) }()
	defer srv.Close()
	deadline := time.Now().Add(3 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", address, 50*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("server did not listen")
		}
		time.Sleep(10 * time.Millisecond)
	}
	requestDone := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get("http://" + address)
		if resp != nil {
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("inflight status %d", resp.StatusCode)
			}
		}
		requestDone <- err
	}()
	select {
	case <-handler.entered:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case <-handler.stopping:
	case <-time.After(3 * time.Second):
		t.Fatal("readiness was not removed")
	}
	select {
	case err := <-result:
		t.Fatalf("returned before draining request: %v", err)
	default:
	}
	close(handler.release)
	if err := <-requestDone; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not finish")
	}
	select {
	case <-handler.stopped:
	default:
		t.Fatal("workers were not drained")
	}
}

func TestServeReportsListenFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	srv := &http.Server{Addr: listener.Addr().String(), Handler: http.NotFoundHandler()}
	if err := serve(context.Background(), srv, srv.Handler); err == nil {
		t.Fatal("listen failure was swallowed")
	}
}
