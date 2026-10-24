package worker

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunnerRunsJobsAndStopsOnCancel(t *testing.T) {
	var okRuns, failRuns atomic.Int32
	r := NewRunner(slog.New(slog.DiscardHandler))
	r.Register(Job{Name: "ok", Interval: 10 * time.Millisecond, Run: func(context.Context) error { okRuns.Add(1); return nil }})
	r.Register(Job{Name: "fails", Interval: 10 * time.Millisecond, Run: func(context.Context) error { failRuns.Add(1); return errors.New("boom") }})

	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() { r.Run(ctx); close(done) }()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}
	if okRuns.Load() < 2 || failRuns.Load() < 2 {
		t.Fatalf("expected repeated runs, got ok=%d fails=%d (a failing job must keep being retried)", okRuns.Load(), failRuns.Load())
	}
}
