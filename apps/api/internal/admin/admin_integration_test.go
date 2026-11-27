//go:build integration

package admin_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/admin"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/content"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testdb"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/seed"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/users"
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

func TestValidateAndPublishWorkflow(t *testing.T) {
	pool := testdb.New(t)
	if _, err := seed.Run(ctx, pool); err != nil {
		t.Fatal(err)
	}
	b, err := content.Load("../../../../content")
	if err != nil {
		t.Fatal(err)
	}
	// Import as drafts so we can validate and publish from the admin service.
	if _, err := content.Import(ctx, pool, b, false); err != nil {
		t.Fatal(err)
	}
	actor, err := users.NewStore(pool).Provision(ctx, users.ProvisionInput{
		Provider: "asgardeo", ProviderSubject: uuid.NewString(), Login: "admin-actor", Name: "Admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	svc := admin.NewService(pool)
	lessons, err := svc.Content(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var target admin.LessonContent
	for _, l := range lessons {
		if l.LessonID == "workloads/scaling-deployments" {
			target = l
			break
		}
	}
	if len(target.Versions) == 0 {
		t.Fatal("scaling-deployments version not imported")
	}
	vID := target.Versions[0].ID

	// Validate replays the hidden reference solution and passes.
	report, err := svc.Validate(ctx, vID)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !report.Valid || !report.SolutionReplay.Passed || len(report.SolutionReplay.Goals) == 0 {
		t.Fatalf("expected valid report, got %+v", report)
	}

	// Unpublishing a draft fails with ErrConflict.
	if _, err := svc.Publish(ctx, actor.UserID, vID, "unpublish"); !errors.Is(err, admin.ErrConflict) {
		t.Fatalf("unpublish draft: want ErrConflict, got %v", err)
	}

	// Publish draft -> published and current.
	pub, err := svc.Publish(ctx, actor.UserID, vID, "publish")
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if pub.CurrentVersionID == nil || *pub.CurrentVersionID != vID || pub.Versions[0].Status != "published" {
		t.Fatalf("after publish: %+v", pub)
	}

	// Unpublish -> retired and current cleared.
	unpub, err := svc.Publish(ctx, actor.UserID, vID, "unpublish")
	if err != nil {
		t.Fatalf("unpublish: %v", err)
	}
	if unpub.CurrentVersionID != nil || unpub.Versions[0].Status != "retired" {
		t.Fatalf("after unpublish: %+v", unpub)
	}

	// Re-publishing a retired version works cleanly without changing published_at.
	repub, err := svc.Publish(ctx, actor.UserID, vID, "publish")
	if err != nil {
		t.Fatalf("republish: %v", err)
	}
	if repub.CurrentVersionID == nil || *repub.CurrentVersionID != vID || repub.Versions[0].Status != "published" {
		t.Fatalf("after republish: %+v", repub)
	}

	// Audit log recorded all three actions attributed to actor.
	entries, _, err := svc.Audit(ctx, nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	var actions []string
	for _, e := range entries {
		if e.ActorUserID != nil && *e.ActorUserID == actor.UserID {
			actions = append(actions, e.Action)
		}
	}
	if len(actions) != 3 || actions[0] != "content.published" || actions[1] != "content.unpublished" || actions[2] != "content.published" {
		t.Fatalf("unexpected audit actions: %v", actions)
	}
}
