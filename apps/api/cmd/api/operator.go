package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/admin"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/config"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/database"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/logging"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/seed"
)

// runOperator runs database maintenance commands. They need only the base configuration.
func runOperator(cmd string, args []string) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("invalid configuration:\n%w", err)
	}
	log := logging.New(os.Stdout, cfg.LogLevel)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	pool, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	switch cmd {
	case "migrate":
		if len(args) != 1 {
			return errors.New("usage: api migrate up|down|reset|status|version")
		}
		m, err := database.NewMigrator(pool, log)
		if err != nil {
			return err
		}
		defer func() { _ = m.Close() }()
		switch args[0] {
		case "up":
			res, err := m.Up(ctx)
			log.Info("migrations applied", "count", len(res))
			return err
		case "down", "reset":
			if cfg.Env == config.EnvProduction {
				return errors.New("down migrations are disabled in production; ship a new forward migration instead")
			}
			if args[0] == "down" {
				_, err := m.Down(ctx)
				return err
			}
			_, err := m.DownTo(ctx, 0)
			return err
		case "status":
			statuses, err := m.Status(ctx)
			if err != nil {
				return err
			}
			for _, s := range statuses {
				applied := "pending"
				if !s.AppliedAt.IsZero() {
					applied = s.AppliedAt.UTC().Format(time.RFC3339)
				}
				_, _ = fmt.Fprintf(os.Stdout, "%-6d %-40s %s\n", s.Source.Version, s.Source.Path, applied)
			}
			return nil
		case "version":
			v, err := m.Version(ctx)
			_, _ = fmt.Fprintln(os.Stdout, v)
			return err
		default:
			return fmt.Errorf("unknown migrate command %q", args[0])
		}

	case "seed", "bootstrap":
		if cmd == "bootstrap" {
			m, err := database.NewMigrator(pool, log)
			if err != nil {
				return err
			}
			res, err := m.Up(ctx)
			_ = m.Close()
			if err != nil {
				return err
			}
			log.Info("migrations applied", "count", len(res))
		}
		c, err := seed.Run(ctx, pool)
		if err != nil {
			return err
		}
		log.Info("seed complete", "courses", c.Courses, "lessons", c.Lessons, "achievements", c.Achievements)
		return nil

	case "admin":
		if len(args) == 0 || args[0] != "set-role" {
			return errors.New("usage: api admin set-role -handle <handle> -role admin|learner -reason <text>")
		}
		fs := flag.NewFlagSet("set-role", flag.ContinueOnError)
		handle := fs.String("handle", "", "user handle")
		role := fs.String("role", "", "admin or learner")
		reason := fs.String("reason", "", "why (recorded in the audit log)")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *handle == "" || *reason == "" {
			return errors.New("-handle and -reason are required")
		}
		id, old, err := admin.SetRole(ctx, pool, *handle, auth.Role(*role), *reason)
		if err != nil {
			return err
		}
		log.Info("role changed", "user_id", id, "from", old, "to", *role)
		return nil
	}
	return fmt.Errorf("unknown command %q", cmd)
}
