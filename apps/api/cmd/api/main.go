// Command api runs the KubeDojo HTTP API.
//
// Usage:
//
//	api [serve]                                    start the HTTP server (default)
//	api healthcheck                                exit 0 if the local liveness probe answers
//	api migrate up|status|version                  apply or inspect database migrations
//	api migrate down|reset                         roll back one / all migrations (not in production)
//	api seed                                       upsert reference data (idempotent)
//	api bootstrap                                  migrate up, then seed (container start-up)
//	api content import [-publish] DIR              import validated lesson content (operator)
//	api admin set-role -handle H -role R -reason T change a user's role (audited)
package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	// Embedded IANA time zone database: the distroless image has none, and user settings
	// validate time zones.
	_ "time/tzdata"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/account"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/admin"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/courses"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/health"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/httpapi"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/labs"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/buildinfo"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/config"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/database"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/httpserver"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/httpx"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/logging"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/ratelimit"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/redis"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/progress"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/users"
)

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	var err error
	switch cmd {
	case "serve":
		err = serve()
	case "healthcheck":
		err = healthcheck()
	case "migrate", "seed", "bootstrap", "admin", "content":
		err = runOperator(cmd, os.Args[2:])
	default:
		err = fmt.Errorf("unknown command %q (want serve, healthcheck, migrate, seed, bootstrap, content or admin)", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "api:", err)
		os.Exit(1)
	}
}

func serve() error {
	cfg, err := config.LoadAPI(os.Getenv)
	if err != nil {
		return fmt.Errorf("invalid configuration:\n%w", err)
	}
	log := logging.New(os.Stdout, cfg.LogLevel)
	log.Info("starting api", "version", buildinfo.Version, "commit", buildinfo.Commit, "env", cfg.Env, "jwt_kids", len(cfg.JWTPublicKeys))
	slog.SetDefault(log)

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

	userStore := users.NewStore(pool)
	// Pagination cursors are signed per process; a restart only invalidates cursors in flight.
	cursorKey := make([]byte, 32)
	if _, err := rand.Read(cursorKey); err != nil {
		return err
	}
	router := httpapi.NewRouter(httpapi.Deps{
		Log: log,
		Health: health.Handler{
			Postgres: health.PingFunc(pool.Ping),
			Redis:    health.PingFunc(func(ctx context.Context) error { return rdb.Ping(ctx).Err() }),
		},
		Verifier:       auth.NewVerifier(cfg.JWTPublicKeys, nil),
		Identities:     userStore,
		Users:          userStore,
		Catalog:        courses.NewService(pool),
		Progress:       progress.NewService(pool),
		Labs:           labs.NewService(pool),
		Accounts:       account.NewService(pool),
		Admin:          admin.NewService(pool),
		Settings:       userStore,
		Cursors:        httpx.NewCursors(cursorKey),
		Limiter:        ratelimit.NewRedis(rdb),
		RequestTimeout: cfg.RequestTimeout,
		MaxBodyBytes:   cfg.MaxBodyBytes,
	})
	if err := httpserver.Run(ctx, log, cfg.HTTPAddr, router, cfg.ShutdownTimeout); err != nil {
		return err
	}
	log.Info("api stopped")
	return nil
}

// healthcheck probes the liveness endpoint of the local process. Distroless images have no
// curl, so the binary checks itself. Only the port is taken from API_LISTEN_ADDR; the host is
// always loopback.
func healthcheck() error {
	addr := os.Getenv("API_LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid API_LISTEN_ADDR: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid port in API_LISTEN_ADDR")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/api/v1/healthz", port), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthz returned %d", resp.StatusCode)
	}
	return nil
}
