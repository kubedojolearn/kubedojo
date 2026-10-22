// Package worker schedules periodic background jobs with graceful shutdown.
package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Job is one periodic task. Run must return promptly when ctx is cancelled.
type Job struct {
	Name     string
	Interval time.Duration
	Run      func(ctx context.Context) error
}

// Runner runs registered jobs on their intervals until its context is cancelled.
type Runner struct {
	log  *slog.Logger
	jobs []Job
}

// NewRunner returns a Runner with no jobs.
func NewRunner(log *slog.Logger) *Runner { return &Runner{log: log} }

// Register adds a job. It must be called before Run.
func (r *Runner) Register(j Job) { r.jobs = append(r.jobs, j) }

// Run starts every job (once immediately, then every Interval) and blocks until ctx is cancelled
// and all in-flight runs have returned. A failing run is logged and retried on the next tick.
func (r *Runner) Run(ctx context.Context) {
	if len(r.jobs) == 0 {
		r.log.Info("no background jobs registered; idling until shutdown")
	}
	var wg sync.WaitGroup
	for _, j := range r.jobs {
		wg.Go(func() { r.loop(ctx, j) })
	}
	<-ctx.Done()
	wg.Wait()
}

func (r *Runner) loop(ctx context.Context, j Job) {
	ticker := time.NewTicker(j.Interval)
	defer ticker.Stop()
	for {
		start := time.Now()
		if err := j.Run(ctx); err != nil && ctx.Err() == nil {
			r.log.Error("job failed", "job", j.Name, "error", err.Error(), "duration_ms", time.Since(start).Milliseconds())
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
