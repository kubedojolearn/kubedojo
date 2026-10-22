//go:build integration

package database_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/database"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testdb"
)

var ctx = context.Background()

func tableCount(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name <> 'goose_db_version'`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestMigrationsUpDownUp(t *testing.T) {
	pool := testdb.NewEmpty(t)
	m, err := database.NewMigrator(pool, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Close() }()

	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("up: %v", err)
	}
	if n := tableCount(t, pool); n != 21 {
		t.Fatalf("expected 21 tables after up, got %d", n)
	}
	if v, _ := m.Version(ctx); v != 5 {
		t.Fatalf("version %d", v)
	}
	if _, err := m.DownTo(ctx, 0); err != nil {
		t.Fatalf("down: %v", err)
	}
	if n := tableCount(t, pool); n != 0 {
		t.Fatalf("expected empty schema after down, got %d tables", n)
	}
	var fns int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace WHERE n.nspname = 'public'`).Scan(&fns)
	if fns != 0 {
		t.Fatalf("down left %d functions behind", fns)
	}
	if _, err := m.Up(ctx); err != nil {
		t.Fatalf("up again: %v", err)
	}
	statuses, err := m.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range statuses {
		if s.AppliedAt.IsZero() {
			t.Fatalf("migration %d not applied", s.Source.Version)
		}
	}
}

func expectViolation(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected SQLSTATE %s, got success", code)
	}
	if !database.IsCode(err, code) {
		t.Fatalf("expected SQLSTATE %s, got %v", code, err)
	}
}

func TestIdentityConstraints(t *testing.T) {
	pool := testdb.New(t)
	q := dbgen.New(pool)
	a := testdb.CreateUser(t, pool, "Octo-Cat")

	// Handles are unique case-insensitively.
	_, err := q.InsertUser(ctx, dbgen.InsertUserParams{Handle: "octo-cat", DisplayName: "x"})
	expectViolation(t, err, database.CodeUniqueViolation)
	_, err = q.InsertUser(ctx, dbgen.InsertUserParams{Handle: "-bad-", DisplayName: "x"})
	expectViolation(t, err, database.CodeCheckViolation)

	// One local user per provider identity; a second user can't claim it.
	lone, err := q.InsertUser(ctx, dbgen.InsertUserParams{Handle: "no-identity-yet", DisplayName: "x"})
	if err != nil {
		t.Fatal(err)
	}
	b := testdb.CreateUser(t, pool, "second")
	if err := q.InsertIdentity(ctx, dbgen.InsertIdentityParams{Provider: "github", ProviderSubject: "583231", UserID: lone, ProviderLogin: "a"}); err != nil {
		t.Fatal(err)
	}
	// And one identity per provider per user.
	err = q.InsertIdentity(ctx, dbgen.InsertIdentityParams{Provider: "github", ProviderSubject: "999", UserID: a, ProviderLogin: "a"})
	expectViolation(t, err, database.CodeUniqueViolation)
	err = q.InsertIdentity(ctx, dbgen.InsertIdentityParams{Provider: "github", ProviderSubject: "583231", UserID: b, ProviderLogin: "b"})
	expectViolation(t, err, database.CodeUniqueViolation)
	err = q.InsertIdentity(ctx, dbgen.InsertIdentityParams{Provider: "github", ProviderSubject: "octocat", UserID: b, ProviderLogin: "b"})
	expectViolation(t, err, database.CodeCheckViolation)

	// Roles are constrained and default to learner.
	var role string
	_ = pool.QueryRow(ctx, "SELECT role FROM users WHERE id = $1", b).Scan(&role)
	if role != "learner" {
		t.Fatalf("default role %q", role)
	}
	_, err = pool.Exec(ctx, "UPDATE users SET role = 'superuser' WHERE id = $1", b)
	expectViolation(t, err, database.CodeCheckViolation)

	// Privacy defaults are conservative.
	var public, leaderboard bool
	_ = pool.QueryRow(ctx, "SELECT profile_public, leaderboard_opt_in FROM user_settings WHERE user_id = $1", b).Scan(&public, &leaderboard)
	if public || leaderboard {
		t.Fatal("profiles and leaderboard must default to off")
	}
}

func TestChallengeVersionsAreImmutableOncePublished(t *testing.T) {
	pool := testdb.New(t)
	q := dbgen.New(pool)
	l := testdb.CreatePublishedLesson(t, pool, "scale")

	_, err := pool.Exec(ctx, `UPDATE challenge_versions SET spec = '{"tampered":true}' WHERE id = $1`, l.VersionID)
	expectViolation(t, err, database.CodeIntegrityViolation)
	_, err = pool.Exec(ctx, `UPDATE challenge_versions SET status = 'draft', published_at = NULL WHERE id = $1`, l.VersionID)
	if err == nil {
		t.Fatal("published version went back to draft")
	}
	_, err = pool.Exec(ctx, `DELETE FROM challenge_versions WHERE id = $1`, l.VersionID)
	expectViolation(t, err, database.CodeIntegrityViolation)

	// Retiring is allowed bookkeeping.
	if _, err := pool.Exec(ctx, `UPDATE lessons SET current_challenge_version_id = NULL WHERE id = $1`, l.LessonID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE challenge_versions SET status = 'retired', retired_at = now() WHERE id = $1`, l.VersionID); err != nil {
		t.Fatalf("retire: %v", err)
	}

	// Drafts are editable and deletable.
	draft := testdb.InsertVersion(t, pool, l.LessonID, 2, `{"v":2}`)
	if _, err := pool.Exec(ctx, `UPDATE challenge_versions SET spec = '{"v":"2b"}' WHERE id = $1`, draft); err != nil {
		t.Fatalf("edit draft: %v", err)
	}

	// Re-importing identical content is a no-op; versions are unique per lesson.
	sameContent, err := q.InsertChallengeVersion(ctx, dbgen.InsertChallengeVersionParams{
		LessonID: l.LessonID, Version: 9, ContentSha256: "0000000000000000000000000000000000000000000000000000000000000000",
		Spec: json.RawMessage(`{}`), SimulatorVersion: testdb.SimulatorVersion,
	})
	if err != nil || sameContent == uuid.Nil {
		t.Fatalf("insert: %v", err)
	}
	_, err = q.InsertChallengeVersion(ctx, dbgen.InsertChallengeVersionParams{
		LessonID: l.LessonID, Version: 10, ContentSha256: "0000000000000000000000000000000000000000000000000000000000000000",
		Spec: json.RawMessage(`{}`), SimulatorVersion: testdb.SimulatorVersion,
	})
	if err != pgx.ErrNoRows {
		t.Fatalf("duplicate content must be skipped, got %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO challenge_versions (lesson_id, version, content_sha256, spec, simulator_version) VALUES ($1, 2, repeat('a', 64), '{}', 'x')`, l.LessonID)
	expectViolation(t, err, database.CodeUniqueViolation)
}

func TestLessonCurrentVersionMustBePublishedAndOwn(t *testing.T) {
	pool := testdb.New(t)
	a := testdb.CreatePublishedLesson(t, pool, "a")
	b := testdb.CreatePublishedLesson(t, pool, "b")
	draft := testdb.InsertVersion(t, pool, a.LessonID, 2, `{"draft":true}`)

	_, err := pool.Exec(ctx, `UPDATE lessons SET current_challenge_version_id = $1 WHERE id = $2`, draft, a.LessonID)
	expectViolation(t, err, database.CodeIntegrityViolation)
	_, err = pool.Exec(ctx, `UPDATE lessons SET current_challenge_version_id = $1 WHERE id = $2`, b.VersionID, a.LessonID)
	expectViolation(t, err, database.CodeIntegrityViolation)
}

func TestOneActiveLabPerLessonAndLifecycleChecks(t *testing.T) {
	pool := testdb.New(t)
	user := testdb.CreateUser(t, pool, "learner")
	l := testdb.CreatePublishedLesson(t, pool, "scale")
	first := testdb.CreateLab(t, pool, user, l)

	_, err := dbgen.New(pool).CreateLabSession(ctx, dbgen.CreateLabSessionParams{
		UserID: user, LessonID: l.LessonID, ChallengeVersionID: l.VersionID, SimulatorVersion: "x",
		Seed: 1, SimTime: first.SimTime, ExpiresAt: first.ExpiresAt,
	})
	expectViolation(t, err, database.CodeUniqueViolation)

	// Ending a lab requires ended_at; then a new active lab may start.
	_, err = pool.Exec(ctx, `UPDATE lab_sessions SET state = 'completed' WHERE id = $1`, first.ID)
	expectViolation(t, err, database.CodeCheckViolation)
	if _, err := pool.Exec(ctx, `UPDATE lab_sessions SET state = 'completed', ended_at = now() WHERE id = $1`, first.ID); err != nil {
		t.Fatal(err)
	}
	second := testdb.CreateLab(t, pool, user, l)
	if second.ID == first.ID {
		t.Fatal("expected a new lab")
	}
	// Lab IDs are random v4, not time-ordered v7.
	if second.ID.Version() != 4 {
		t.Fatalf("lab id version %d", second.ID.Version())
	}

	// Resource limits and keys.
	_, err = pool.Exec(ctx, `INSERT INTO lab_resources (session_id, kind, namespace, name, api_version, uid, resource_version, metadata)
		VALUES ($1, 'Pod', 'default', 'web', 'v1', gen_random_uuid(), 1, '{}'), ($1, 'Pod', 'default', 'web', 'v1', gen_random_uuid(), 1, '{}')`, second.ID)
	expectViolation(t, err, database.CodeUniqueViolation)
	_, err = pool.Exec(ctx, `INSERT INTO lab_files (session_id, path, content) VALUES ($1, '../etc/passwd', 'x')`, second.ID)
	expectViolation(t, err, database.CodeCheckViolation)
	_, err = pool.Exec(ctx, `INSERT INTO lab_answers (session_id, goal_id, submission_no, answer) VALUES ($1, 'node-count', 11, '3')`, second.ID)
	expectViolation(t, err, database.CodeCheckViolation)
	_, err = pool.Exec(ctx, `INSERT INTO command_executions (session_id, sequence, request_id, input, exit_code, mutated, snapshot_version, duration_ms)
		VALUES ($1, 1, gen_random_uuid(), repeat('a', 16385), 0, false, 1, 0)`, second.ID)
	expectViolation(t, err, database.CodeCheckViolation)
}

func TestCommandsAreDeduplicatedByRequestID(t *testing.T) {
	pool := testdb.New(t)
	lab := testdb.CreateLab(t, pool, testdb.CreateUser(t, pool, "u"), testdb.CreatePublishedLesson(t, pool, "x"))
	req := uuid.New()
	insert := `INSERT INTO command_executions (session_id, sequence, request_id, input, exit_code, mutated, snapshot_version, duration_ms)
		VALUES ($1, $2, $3, 'kubectl get pods', 0, false, 1, 3)`
	if _, err := pool.Exec(ctx, insert, lab.ID, 1, req); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, insert, lab.ID, 2, req)
	expectViolation(t, err, database.CodeUniqueViolation)
}

func TestXPLedgerIsAppendOnly(t *testing.T) {
	pool := testdb.New(t)
	q := dbgen.New(pool)
	user := testdb.CreateUser(t, pool, "u")
	params := dbgen.InsertXPEventParams{UserID: user, EventKey: "lesson-complete:abc", Delta: 100, Reason: "lesson_completed"}
	if _, err := q.InsertXPEvent(ctx, params); err != nil {
		t.Fatal(err)
	}
	if _, err := q.InsertXPEvent(ctx, params); err != pgx.ErrNoRows {
		t.Fatalf("duplicate event key must be ignored, got %v", err)
	}
	_, err := pool.Exec(ctx, `INSERT INTO xp_events (user_id, event_key, delta, reason) VALUES ($1, 'lesson-complete:abc', 5, 'adjustment')`, user)
	expectViolation(t, err, database.CodeUniqueViolation)
	_, err = pool.Exec(ctx, `UPDATE xp_events SET delta = 9999 WHERE user_id = $1`, user)
	expectViolation(t, err, database.CodeIntegrityViolation)
	_, err = pool.Exec(ctx, `DELETE FROM xp_events WHERE user_id = $1`, user)
	expectViolation(t, err, database.CodeIntegrityViolation)
	_, err = pool.Exec(ctx, `INSERT INTO xp_events (user_id, event_key, delta, reason) VALUES ($1, 'k:zero', 0, 'adjustment')`, user)
	expectViolation(t, err, database.CodeCheckViolation)
	_, err = pool.Exec(ctx, `UPDATE user_totals SET total_xp = -1 WHERE user_id = $1`, user)
	expectViolation(t, err, database.CodeCheckViolation)
}

func TestAuditLogIsAppendOnly(t *testing.T) {
	pool := testdb.New(t)
	id, err := dbgen.New(pool).InsertAuditLog(ctx, dbgen.InsertAuditLogParams{
		ActorKind: "operator", Action: "user.role_changed", EntityType: "user", EntityID: "x", Details: json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `UPDATE audit_logs SET action = 'user.deleted' WHERE id = $1`, id)
	expectViolation(t, err, database.CodeIntegrityViolation)
	_, err = pool.Exec(ctx, `DELETE FROM audit_logs WHERE id = $1`, id)
	expectViolation(t, err, database.CodeIntegrityViolation)
}

func TestUserErasureCascadesButKeepsCurriculum(t *testing.T) {
	pool := testdb.New(t)
	user := testdb.CreateUser(t, pool, "leaver")
	l := testdb.CreatePublishedLesson(t, pool, "keep")
	lab := testdb.CreateLab(t, pool, user, l)
	q := dbgen.New(pool)
	if _, err := q.InsertXPEvent(ctx, dbgen.InsertXPEventParams{UserID: user, EventKey: "lesson-complete:k", Delta: 10, Reason: "lesson_completed"}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.InsertAuditLog(ctx, dbgen.InsertAuditLogParams{ActorUserID: &user, ActorKind: "user", Action: "user.settings_changed", EntityType: "user", EntityID: user.String(), Details: json.RawMessage(`{}`)}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO lesson_progress (user_id, lesson_id, first_started_at) VALUES ($1, $2, now())`, user, l.LessonID); err != nil {
		t.Fatal(err)
	}

	// Without the erasure flag the ledger protects itself.
	_, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, user)
	expectViolation(t, err, database.CodeIntegrityViolation)

	err = database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT set_config('kubedojo.erasing_user', $1, true)`, user.String()); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `DELETE FROM users WHERE id = $1`, user)
		return err
	})
	if err != nil {
		t.Fatalf("erase: %v", err)
	}
	for _, check := range []struct{ table, where string }{
		{"lab_sessions", "id = $1"}, {"oauth_identities", "user_id = $1"}, {"user_settings", "user_id = $1"},
		{"xp_events", "user_id = $1"}, {"lesson_progress", "user_id = $1"}, {"user_totals", "user_id = $1"},
	} {
		arg := user
		if check.table == "lab_sessions" {
			arg = lab.ID
		}
		var n int
		_ = pool.QueryRow(ctx, "SELECT count(*) FROM "+check.table+" WHERE "+check.where, arg).Scan(&n)
		if n != 0 {
			t.Errorf("%s not erased", check.table)
		}
	}
	var lessons, versions, audit int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM lessons WHERE id = $1`, l.LessonID).Scan(&lessons)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM challenge_versions WHERE id = $1`, l.VersionID).Scan(&versions)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE entity_id = $1 AND actor_user_id IS NULL`, user.String()).Scan(&audit)
	if lessons != 1 || versions != 1 {
		t.Fatal("curriculum must survive user deletion")
	}
	if audit != 1 {
		t.Fatal("audit entry must remain with the actor link removed")
	}
}

func TestCoursesWithLessonsCannotBeDeleted(t *testing.T) {
	pool := testdb.New(t)
	l := testdb.CreatePublishedLesson(t, pool, "x")
	_, err := pool.Exec(ctx, `DELETE FROM courses WHERE id = $1`, l.CourseID)
	expectViolation(t, err, database.CodeRestrictViolation)
}
