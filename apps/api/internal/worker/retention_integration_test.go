//go:build integration

package worker_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/content"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/labs"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testdb"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/seed"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/users"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/worker"
)

func TestRetentionJobs(t *testing.T) {
	ctx := context.Background()
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
	u, err := users.NewStore(pool).Provision(ctx, users.ProvisionInput{
		Provider: "asgardeo", ProviderSubject: uuid.NewString(), Login: "ret-learner", Name: "Retention",
	})
	if err != nil {
		t.Fatal(err)
	}

	var versionID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT current_challenge_version_id FROM lessons WHERE current_challenge_version_id IS NOT NULL LIMIT 1`).Scan(&versionID); err != nil {
		t.Fatal(err)
	}
	labSvc := labs.NewService(pool)
	oldLab, _, err := labSvc.Create(ctx, u.UserID, versionID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := labSvc.RunCommand(ctx, u.UserID, oldLab.ID, uuid.New(), "kubectl get pods"); err != nil {
		t.Fatal(err)
	}
	if _, err := labSvc.Finish(ctx, u.UserID, oldLab.ID, "finish-old-lab-1"); err != nil {
		t.Fatal(err)
	}
	// Backdate ended_at beyond 30 days.
	if _, err := pool.Exec(ctx, `UPDATE lab_sessions SET ended_at = now() - interval '31 days' WHERE id = $1`, oldLab.ID); err != nil {
		t.Fatal(err)
	}

	// Start a second, active lab whose data must NOT be purged.
	activeLab, _, err := labSvc.Create(ctx, u.UserID, versionID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := labSvc.RunCommand(ctx, u.UserID, activeLab.ID, uuid.New(), "kubectl get pods"); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	purged, err := worker.PurgeEndedLabContent(ctx, pool, now, 100)
	if err != nil || purged != 1 {
		t.Fatalf("PurgeEndedLabContent: purged=%d err=%v", purged, err)
	}
	var oldResources, activeResources int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM lab_resources WHERE session_id = $1`, oldLab.ID).Scan(&oldResources)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM lab_resources WHERE session_id = $1`, activeLab.ID).Scan(&activeResources)
	if oldResources != 0 || activeResources == 0 {
		t.Fatalf("resources: old=%d active=%d", oldResources, activeResources)
	}
	// Session row itself is kept.
	var sessionStillExists bool
	_ = pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM lab_sessions WHERE id = $1)`, oldLab.ID).Scan(&sessionStillExists)
	if !sessionStillExists {
		t.Fatal("ended lab_sessions row was unexpectedly deleted")
	}

	// Command history cap (90 days).
	if _, err := pool.Exec(ctx, `UPDATE command_executions SET executed_at = now() - interval '91 days' WHERE session_id = $1`, activeLab.ID); err != nil {
		t.Fatal(err)
	}
	prunedCmds, err := worker.PruneCommandHistory(ctx, pool, now, 100)
	if err != nil || prunedCmds != 1 {
		t.Fatalf("PruneCommandHistory: n=%d err=%v", prunedCmds, err)
	}

	// Outbox cleanup (7 days for published rows; unpublished rows stay).
	if _, err := pool.Exec(ctx, `
		INSERT INTO outbox_events (event_type, aggregate_type, aggregate_id, payload, published_at)
		VALUES ('lab.state_changed', 'lab', $1, '{}', now() - interval '8 days'),
		       ('lab.state_changed', 'lab', $1, '{}', NULL)`, activeLab.ID); err != nil {
		t.Fatal(err)
	}
	prunedOutbox, err := worker.PrunePublishedOutbox(ctx, pool, now, 100)
	if err != nil || prunedOutbox != 1 {
		t.Fatalf("PrunePublishedOutbox: n=%d err=%v", prunedOutbox, err)
	}

	// Activity retention (180 days).
	if _, err := pool.Exec(ctx, `
		INSERT INTO activity_events (user_id, type, occurred_at)
		VALUES ($1, 'lab-started', now() - interval '181 days'),
		       ($1, 'lab-started', now() - interval '10 days')`, u.UserID); err != nil {
		t.Fatal(err)
	}
	prunedAct, err := worker.PruneActivityEvents(ctx, pool, now, 100)
	if err != nil || prunedAct != 1 {
		t.Fatalf("PruneActivityEvents: n=%d err=%v", prunedAct, err)
	}

	// Audit log retention (1 year, trigger-enforced).
	if _, err := pool.Exec(ctx, `
		INSERT INTO audit_logs (actor_kind, action, entity_type, entity_id, created_at)
		VALUES ('operator', 'content.published', 'lesson', 'old', now() - interval '400 days'),
		       ('operator', 'content.published', 'lesson', 'new', now() - interval '10 days')`); err != nil {
		t.Fatal(err)
	}
	prunedAudit, err := worker.PruneAuditLogs(ctx, pool, now, 100)
	if err != nil || prunedAudit != 1 {
		t.Fatalf("PruneAuditLogs: n=%d err=%v", prunedAudit, err)
	}
}
