package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

// TestProjectionCancelStopsGoroutine verifies that cancelling a context
// propagates into a long-running goroutine, which is the pattern used
// for the projection updater.
func TestProjectionCancelStopsGoroutine(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		// Simulate a blocking projection updater loop
		<-ctx.Done()
	}()

	cancel()

	select {
	case <-stopped:
		// success: goroutine exited
	case <-time.After(2 * time.Second):
		t.Fatal("projection goroutine did not stop after context cancellation")
	}
}

// TestDebugServerShutdown verifies that calling Shutdown on an http.Server
// causes Serve to return http.ErrServerClosed, which is the pattern used
// for the debug server.
func TestDebugServerShutdown(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		Handler: mux,
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.Serve(ln)
	}()

	// Give the server a moment to start
	time.Sleep(50 * time.Millisecond)

	// Shut it down
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}

	select {
	case err := <-serverErr:
		if err != http.ErrServerClosed {
			t.Fatalf("expected ErrServerClosed, got: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not stop after Shutdown")
	}
}

// TestUserSyncTimeout verifies that a goroutine using context.WithTimeout
// is cancelled after the deadline, preventing it from blocking forever.
func TestUserSyncTimeout(t *testing.T) {
	done := make(chan struct{})

	go func() {
		defer close(done)

		syncCtx, syncCancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer syncCancel()

		// Simulate a slow external call that blocks until context expires
		<-syncCtx.Done()
	}()

	select {
	case <-done:
		// success: goroutine exited within the timeout
	case <-time.After(2 * time.Second):
		t.Fatal("user sync goroutine did not respect timeout")
	}
}
