package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"
)

func newTestHTTPListener(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	return listener
}

func waitHTTPTestSignal(t *testing.T, signal <-chan struct{}, description string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", description)
	}
}

func TestServerShutdownWaitsForActiveRequest(t *testing.T) {
	listener := newTestHTTPListener(t)
	requestStarted := make(chan struct{})
	releaseRequest := make(chan struct{})
	shutdownStarted := make(chan struct{})
	server := NewServer(listener.Addr().String(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-releaseRequest
		w.WriteHeader(http.StatusOK)
	}))
	server.RegisterOnShutdown(func() { close(shutdownStarted) })
	ctx, cancel := context.WithCancel(context.Background())
	released := false
	t.Cleanup(func() {
		cancel()
		if !released {
			close(releaseRequest)
		}
		_ = server.Close()
	})

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- serve(ctx, server, listener, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}()

	transport := &http.Transport{}
	t.Cleanup(transport.CloseIdleConnections)
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	requestDone := make(chan error, 1)
	go func() {
		response, err := client.Get("http://" + listener.Addr().String())
		if err != nil {
			requestDone <- err
			return
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			requestDone <- fmt.Errorf("expected HTTP 200, got %d", response.StatusCode)
			return
		}
		_, err = io.Copy(io.Discard, response.Body)
		requestDone <- err
	}()

	waitHTTPTestSignal(t, requestStarted, "active request")
	cancel()
	waitHTTPTestSignal(t, shutdownStarted, "server shutdown")
	select {
	case err := <-serverDone:
		t.Fatalf("server returned before the active request finished: %v", err)
	default:
	}

	close(releaseRequest)
	released = true
	select {
	case err := <-requestDone:
		if err != nil {
			t.Fatalf("active request must finish successfully: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("active request did not finish")
	}
	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatalf("graceful shutdown failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down")
	}
}

func TestRunServerReturnsListenError(t *testing.T) {
	listener := newTestHTTPListener(t)
	server := NewServer(listener.Addr().String(), http.NotFoundHandler())
	err := RunServer(context.Background(), server, nil)
	var opErr *net.OpError
	if !errors.As(err, &opErr) || opErr.Op != "listen" {
		t.Fatalf("expected listener failure to propagate, got %v", err)
	}
}

func TestRunServerCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	server := NewServer("invalid address", http.NotFoundHandler())
	if err := RunServer(ctx, server, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation before opening the listener, got %v", err)
	}
}
