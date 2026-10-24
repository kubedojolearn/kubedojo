// Package testdb gives integration tests an isolated, fully migrated PostgreSQL database.
// A template database is migrated once (named by a hash of the migrations, so schema changes
// produce a new template) and each test gets a fast copy that is dropped afterwards.
// Import only from _test.go files built with -tags=integration.
package testdb

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kubedojolearn/kubedojo/apps/api/db"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/database"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testenv"
)

// advisoryLockKey serializes template creation across parallel test processes.
const advisoryLockKey = 727274

// New returns a pool connected to a fresh database with all migrations applied.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	adminURL := testenv.Require(t, "TEST_DATABASE_URL")
	tmpl := templateName(t)

	admin := connect(t, adminURL)
	defer func() { _ = admin.Close(ctx) }()
	if _, err := admin.Exec(ctx, "SELECT pg_advisory_lock($1)", advisoryLockKey); err != nil {
		t.Fatal(err)
	}
	ensureTemplate(t, ctx, admin, adminURL, tmpl)
	if _, err := admin.Exec(ctx, "SELECT pg_advisory_unlock($1)", advisoryLockKey); err != nil {
		t.Fatal(err)
	}
	return createDatabase(t, admin, adminURL, "TEMPLATE "+tmpl)
}

// NewEmpty returns a pool to a fresh database with no migrations applied (for migration tests).
func NewEmpty(t testing.TB) *pgxpool.Pool {
	t.Helper()
	adminURL := testenv.Require(t, "TEST_DATABASE_URL")
	admin := connect(t, adminURL)
	defer func() { _ = admin.Close(context.Background()) }()
	return createDatabase(t, admin, adminURL, "")
}

func connect(t testing.TB, rawURL string) *pgx.Conn {
	t.Helper()
	c, err := pgx.Connect(context.Background(), rawURL)
	if err != nil {
		t.Fatalf("connect to test server: %v", err)
	}
	return c
}

func createDatabase(t testing.TB, admin *pgx.Conn, adminURL, options string) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	name := "t_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:20]
	if _, err := admin.Exec(ctx, strings.TrimSpace(fmt.Sprintf("CREATE DATABASE %s %s", name, options))); err != nil {
		t.Fatalf("create test database: %v", err)
	}
	pool, err := database.Open(ctx, withDatabase(t, adminURL, name))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		c := connect(t, adminURL)
		defer func() { _ = c.Close(context.Background()) }()
		if _, err := c.Exec(context.Background(), fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", name)); err != nil {
			t.Errorf("drop test database: %v", err)
		}
	})
	return pool
}

func ensureTemplate(t testing.TB, ctx context.Context, admin *pgx.Conn, adminURL, tmpl string) {
	t.Helper()
	var exists bool
	if err := admin.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)", tmpl).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		return
	}
	building := tmpl + "_building"
	if _, err := admin.Exec(ctx, "DROP DATABASE IF EXISTS "+building+" WITH (FORCE)"); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+building); err != nil {
		t.Fatal(err)
	}
	pool, err := database.Open(ctx, withDatabase(t, adminURL, building))
	if err != nil {
		t.Fatal(err)
	}
	m, err := database.NewMigrator(pool, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("migrate template: %v", err)
	}
	_ = m.Close()
	pool.Close()
	if _, err := admin.Exec(ctx, fmt.Sprintf("ALTER DATABASE %s RENAME TO %s", building, tmpl)); err != nil {
		t.Fatal(err)
	}
}

// templateName hashes the migration files so a changed schema gets a new template.
func templateName(t testing.TB) string {
	t.Helper()
	var names []string
	if err := fs.WalkDir(db.Migrations, ".", func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			names = append(names, p)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, n := range names {
		b, err := db.Migrations.ReadFile(n)
		if err != nil {
			t.Fatal(err)
		}
		h.Write([]byte(n))
		h.Write(b)
	}
	return "kubedojo_tmpl_" + hex.EncodeToString(h.Sum(nil))[:12]
}

func withDatabase(t testing.TB, rawURL, name string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal("invalid TEST_DATABASE_URL")
	}
	u.Path = "/" + name
	return u.String()
}
