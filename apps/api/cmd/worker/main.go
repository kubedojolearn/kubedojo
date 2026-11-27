// Command worker runs KubeDojo background jobs (outbox publishing, session expiry and retention
// as they are added in later parts). It shares configuration and storage with the API.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/account"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/labs"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/buildinfo"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/config"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/database"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/logging"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/observability"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/redis"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/realtime"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/worker"
)

func main() {
	cmd := "run"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	var err error
	switch cmd {
	case "run":
		err = run()
	case "healthcheck":
		err = healthcheck()
	default:
		err = fmt.Errorf("unknown command %q (want run or healthcheck)", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "worker:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("invalid configuration:\n%w", err)
	}
	log := logging.New(os.Stdout, cfg.LogLevel)
	log.Info("starting worker", "version", buildinfo.Version, "commit", buildinfo.Commit, "env", cfg.Env)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	otelProvider, err := observability.Init(ctx, observability.Config{
		ServiceName:    "kubedojo-worker",
		ServiceVersion: buildinfo.Version,
		Environment:    cfg.Env,
		OTLPEndpoint:   cfg.OTLPEndpoint,
	})
	if err != nil {
		return err
	}
	defer func() {
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := otelProvider.Shutdown(shutCtx); err != nil {
			log.Warn("otel shutdown failed", "error", err)
		}
	}()

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	rdb, err := redis.Open(ctx, cfg.RedisURL)
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()

	// Background jobs: outbox relay, lab expiry, account erasure and retention policies
	// (docs/operations/database.md).
	runner := worker.NewRunner(log)
	// Lab state notifications for WebSockets (ADR-0004). Short interval: it is the latency of
	// "another tab changed this lab", and an idle pass is one indexed query.
	runner.Register(worker.Job{Name: "outbox-relay", Interval: 500 * time.Millisecond, Run: func(ctx context.Context) error {
		_, err := realtime.RelayOutbox(ctx, pool, rdb)
		return err
	}})
	runner.Register(worker.Job{Name: "lab-expiry", Interval: time.Minute, Run: func(ctx context.Context) error {
		n, err := labs.ExpireIdle(ctx, pool)
		if n > 0 {
			log.Info("expired idle labs", "count", n)
		}
		return err
	}})
	runner.Register(worker.Job{Name: "idempotency-retention", Interval: time.Hour, Run: func(ctx context.Context) error {
		_, err := labs.PruneIdempotency(ctx, pool, time.Now())
		return err
	}})
	accounts := account.NewService(pool)
	runner.Register(worker.Job{Name: "account-erasure", Interval: time.Hour, Run: func(ctx context.Context) error {
		n, err := accounts.EraseDue(ctx, time.Now(), 100)
		if n > 0 {
			log.Info("erased deleted accounts", "count", n)
		}
		return err
	}})
	runner.Register(worker.Job{Name: "lab-content-purge", Interval: time.Hour, Run: func(ctx context.Context) error {
		n, err := worker.PurgeEndedLabContent(ctx, pool, time.Now(), 200)
		if n > 0 {
			log.Info("purged ended lab content", "sessions", n)
		}
		return err
	}})
	runner.Register(worker.Job{Name: "command-history-retention", Interval: time.Hour, Run: func(ctx context.Context) error {
		n, err := worker.PruneCommandHistory(ctx, pool, time.Now(), 5000)
		if n > 0 {
			log.Info("pruned command history", "rows", n)
		}
		return err
	}})
	runner.Register(worker.Job{Name: "outbox-retention", Interval: time.Hour, Run: func(ctx context.Context) error {
		n, err := worker.PrunePublishedOutbox(ctx, pool, time.Now(), 5000)
		if n > 0 {
			log.Info("pruned published outbox", "rows", n)
		}
		return err
	}})
	runner.Register(worker.Job{Name: "activity-retention", Interval: 6 * time.Hour, Run: func(ctx context.Context) error {
		n, err := worker.PruneActivityEvents(ctx, pool, time.Now(), 5000)
		if n > 0 {
			log.Info("pruned activity events", "rows", n)
		}
		return err
	}})
	runner.Register(worker.Job{Name: "audit-retention", Interval: 6 * time.Hour, Run: func(ctx context.Context) error {
		n, err := worker.PruneAuditLogs(ctx, pool, time.Now(), 1000)
		if n > 0 {
			log.Info("pruned audit logs", "rows", n)
		}
		return err
	}})
	runner.Run(ctx)
	log.Info("worker stopped")
	return nil
}

// healthcheck reports whether the worker's stores are reachable. It runs as its own process
// (the Compose healthcheck), because the distroless image has no shell and the worker no port.
func healthcheck() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("invalid configuration:\n%w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	pool.Close()
	rdb, err := redis.Open(ctx, cfg.RedisURL)
	if err != nil {
		return err
	}
	return rdb.Close()
}
