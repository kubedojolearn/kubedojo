package httpserver

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"
)

func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

func TestRunDrainsInFlightRequestsOnShutdown(t *testing.T) {
	addr := freeAddr(t)
	started := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		time.Sleep(200 * time.Millisecond)
		_, _ = io.WriteString(w, "done")
	})
	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- Run(ctx, slog.New(slog.DiscardHandler), addr, h, 5*time.Second) }()

	var resp *http.Response
	var reqErr error
	reqDone := make(chan struct{})
	go func() {
		defer close(reqDone)
		for i := 0; i < 50; i++ {
			resp, reqErr = http.Get("http://" + addr + "/")
			if reqErr == nil {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}()

	<-started
	cancel()
	<-reqDone
	if reqErr != nil {
		t.Fatalf("request failed: %v", reqErr)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "done" {
		t.Fatalf("in-flight request was not drained, got %q", body)
	}
	if err := <-runErr; err != nil {
		t.Fatalf("Run: %v", err)
	}
}
