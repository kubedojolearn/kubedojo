//go:build integration

package admin_test

import (
	"context"
	"errors"
	"testing"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/admin"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/auth"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testdb"
)

func TestSetRoleIsAudited(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	user := testdb.CreateUser(t, pool, "Maintainer")

	id, old, err := admin.SetRole(ctx, pool, "maintainer", auth.RoleAdmin, "initial admin")
	if err != nil || id != user || old != auth.RoleLearner {
		t.Fatalf("%v %v %v", id, old, err)
	}
	var role string
	_ = pool.QueryRow(ctx, "SELECT role FROM users WHERE id = $1", user).Scan(&role)
	if role != "admin" {
		t.Fatalf("role %q", role)
	}
	entries, err := dbgen.New(pool).ListAuditLogs(ctx, dbgen.ListAuditLogsParams{PageLimit: 10})
	if err != nil || len(entries) != 1 {
		t.Fatalf("audit entries %d %v", len(entries), err)
	}
	e := entries[0]
	if e.Action != "user.role_changed" || e.ActorKind != "operator" || e.EntityID != user.String() ||
		e.OldRef == nil || *e.OldRef != "learner" || e.NewRef == nil || *e.NewRef != "admin" {
		t.Fatalf("audit entry %+v", e)
	}
}

func TestSetRoleRejectsUnknownUsersAndRoles(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	testdb.CreateUser(t, pool, "someone")
	if _, _, err := admin.SetRole(ctx, pool, "nobody", auth.RoleAdmin, "x"); !errors.Is(err, admin.ErrUserNotFound) {
		t.Fatalf("unknown user: %v", err)
	}
	if _, _, err := admin.SetRole(ctx, pool, "someone", auth.Role("root"), "x"); err == nil {
		t.Fatal("unknown role accepted")
	}
	if _, err := pool.Exec(ctx, "UPDATE users SET deleted_at = now() WHERE handle = 'someone'"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := admin.SetRole(ctx, pool, "someone", auth.RoleAdmin, "x"); !errors.Is(err, admin.ErrUserNotFound) {
		t.Fatalf("deleted user promoted: %v", err)
	}
	var n int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM audit_logs").Scan(&n)
	if n != 0 {
		t.Fatal("failed attempts must not leave audit entries")
	}
}
