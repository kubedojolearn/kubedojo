//go:build integration

package admin_test

import (
	"context"
	"testing"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/admin"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/content"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testdb"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/seed"
)

var ctx = context.Background()

func TestContentListsEveryLessonWithItsVersions(t *testing.T) {
	pool := testdb.New(t)
	if _, err := seed.Run(ctx, pool); err != nil {
		t.Fatal(err)
	}
	b, err := content.Load("../../../../content")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := content.Import(ctx, pool, b, true); err != nil {
		t.Fatal(err)
	}
	lessons, err := admin.NewService(pool).Content(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var published int
	for _, l := range lessons {
		if l.Versions == nil {
			t.Fatalf("%s: versions must be an array", l.LessonID)
		}
		for _, v := range l.Versions {
			if len(v.ContentSHA256) != 64 {
				t.Fatalf("%s v%d: sha %q", l.LessonID, v.Version, v.ContentSHA256)
			}
			if v.Status == "published" {
				published++
				if l.CurrentVersionID == nil || *l.CurrentVersionID != v.ID {
					t.Fatalf("%s: published version isn't current", l.LessonID)
				}
			}
		}
	}
	if published == 0 {
		t.Fatal("no published versions listed")
	}
}

func TestAuditPagesNewestFirst(t *testing.T) {
	pool := testdb.New(t)
	for _, action := range []string{"content.published", "user.role_changed", "user.deletion_requested"} {
		if _, err := pool.Exec(ctx, `INSERT INTO audit_logs (actor_kind, action, entity_type, entity_id) VALUES ('operator', $1, 'x', 'y')`, action); err != nil {
			t.Fatal(err)
		}
	}
	svc := admin.NewService(pool)
	first, next, err := svc.Audit(ctx, nil, 2)
	if err != nil || next == nil || len(first) != 2 {
		t.Fatalf("page 1: %v %v %v", first, next, err)
	}
	rest, last, err := svc.Audit(ctx, next, 2)
	if err != nil || last != nil || len(rest) != 1 {
		t.Fatalf("page 2: %v %v %v", rest, last, err)
	}
	if first[0].Action != "user.deletion_requested" || rest[0].Action != "content.published" {
		t.Fatalf("order: %s, %s", first[0].Action, rest[0].Action)
	}
}
