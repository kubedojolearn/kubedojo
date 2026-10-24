//go:build integration

package database

import (
	"context"
	"strings"
	"testing"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testenv"
)

func TestOpenConnectsAndSetsSessionParams(t *testing.T) {
	ctx := context.Background()
	pool, err := Open(ctx, testenv.Require(t, "TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer pool.Close()

	var app, timeout, version string
	if err := pool.QueryRow(ctx, "SELECT current_setting('application_name'), current_setting('statement_timeout'), current_setting('server_version')").Scan(&app, &timeout, &version); err != nil {
		t.Fatal(err)
	}
	if app != "kubedojo-api" || timeout != "10s" {
		t.Fatalf("session params: application_name=%q statement_timeout=%q", app, timeout)
	}
	if !strings.HasPrefix(version, "18") {
		t.Fatalf("expected PostgreSQL 18, got %s", version)
	}
}

func TestOpenErrorDoesNotLeakCredentials(t *testing.T) {
	_, err := Open(context.Background(), "postgres://kubedojo:s3cr3t-pw@127.0.0.1:1/nope?sslmode=disable&connect_timeout=1")
	if err == nil {
		t.Fatal("expected connection error")
	}
	if strings.Contains(err.Error(), "s3cr3t-pw") {
		t.Fatalf("error leaks password: %v", err)
	}
}
