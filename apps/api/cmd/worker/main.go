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

	"github.com/kubedojolearn/kubedojo/apps/api/internal/labs"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/buildinfo"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/config"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/database"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/logging"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/redis"
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

	// Jobs are registered here as their parts land (outbox publishing: Part 14 with the
	// WebSocket gateway; retention: Part 19).
	runner := worker.NewRunner(log)
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
