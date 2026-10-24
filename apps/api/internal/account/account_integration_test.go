//go:build integration

package account_test

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/account"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testdb"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/users"
)

var ctx = context.Background()

type env struct {
	pool  *pgxpool.Pool
	svc   *account.Service
	users *users.Store
}

func setup(t *testing.T) env {
	t.Helper()
	pool := testdb.New(t)
	return env{pool: pool, svc: account.NewService(pool), users: users.NewStore(pool)}
}

func (e env) learner(t *testing.T, subject, login string, xp int32, optIn bool) uuid.UUID {
	t.Helper()
	r, err := e.users.Provision(ctx, users.ProvisionInput{Provider: "github", ProviderSubject: subject, Login: login, Name: login})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Exec(ctx, `UPDATE user_totals SET total_xp = $2 WHERE user_id = $1`, r.UserID, xp); err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Exec(ctx, `UPDATE user_settings SET leaderboard_opt_in = $2 WHERE user_id = $1`, r.UserID, optIn); err != nil {
		t.Fatal(err)
	}
	return r.UserID
}

func TestLeaderboardListsOnlyOptedInLearnersWithSharedRanks(t *testing.T) {
	e := setup(t)
	e.learner(t, "1", "ada", 300, true)
	e.learner(t, "2", "brian", 120, true)
	e.learner(t, "3", "carol", 120, true)
	e.learner(t, "4", "hidden", 900, false)
	e.learner(t, "5", "zero", 0, true)

	page1, next, err := e.svc.Leaderboard(ctx, nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || len(page1) != 2 {
		t.Fatalf("page 1 = %+v next %v", page1, next)
	}
	page2, next2, err := e.svc.Leaderboard(ctx, next, 10)
	if err != nil {
		t.Fatal(err)
	}
	if next2 != nil {
		t.Fatalf("unexpected third page")
	}
	all := append(page1, page2...)
	var got []string
	for _, x := range all {
		got = append(got, x.Handle+":"+strconv.FormatInt(x.Rank, 10))
		if x.Handle == "hidden" {
			t.Fatal("a learner who didn't opt in is listed")
		}
	}
	// Ties share rank 2; the next learner is 4th (standard competition ranking).
	want := map[string]bool{"ada:1": true, "brian:2": true, "carol:2": true, "zero:4": true}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for _, g := range got {
		if !want[g] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestExportHasTheUsersDataAndNobodyElses(t *testing.T) {
	e := setup(t)
	me := e.learner(t, "10", "exporter", 40, true)
	e.learner(t, "11", "someone-else", 50, true)

	doc, err := e.svc.Export(ctx, me)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Profile    struct{ Handle string }
		Identities []struct{ ProviderSubject string }
		Settings   struct{ LeaderboardOptIn bool }
		Totals     struct{ TotalXp int }
		Labs       []any
	}
	if err := json.Unmarshal(doc, &got); err != nil {
		t.Fatal(err)
	}
	if got.Profile.Handle != "exporter" || len(got.Identities) != 1 || got.Identities[0].ProviderSubject != "10" ||
		!got.Settings.LeaderboardOptIn || got.Totals.TotalXp != 40 || got.Labs == nil {
		t.Fatalf("export = %s", doc)
	}
	if strings.Contains(string(doc), "someone-else") {
		t.Fatal("export contains another user's data")
	}
}

func TestDeletionNeedsTheHandleThenHidesAndUnlinksTheAccount(t *testing.T) {
	e := setup(t)
	me := e.learner(t, "20", "leaver", 70, true)

	if err := e.svc.RequestDeletion(ctx, me, "someone"); !errors.Is(err, account.ErrHandleMismatch) {
		t.Fatalf("wrong handle: err = %v", err)
	}
	if _, err := e.users.GetMe(ctx, me); err != nil {
		t.Fatalf("a failed confirmation must not delete anything: %v", err)
	}

	if err := e.svc.RequestDeletion(ctx, me, "LEAVER"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.users.GetMe(ctx, me); !errors.Is(err, users.ErrNotFound) {
		t.Fatalf("deleted user still resolves: %v", err)
	}
	if _, err := e.users.Resolve(ctx, "github", "20"); err == nil {
		t.Fatal("the GitHub identity still signs in to the deleted account")
	}
	board, _, err := e.svc.Leaderboard(ctx, nil, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(board) != 0 {
		t.Fatalf("deleted user is on the leaderboard: %+v", board)
	}
	var actions int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE action = 'user.deletion_requested' AND entity_id = $1`, me.String()).Scan(&actions); err != nil || actions != 1 {
		t.Fatalf("audit entries = %d (%v)", actions, err)
	}

	// Signing in again with the same GitHub account creates a new user; the old handle stays reserved.
	again, err := e.users.Provision(ctx, users.ProvisionInput{Provider: "github", ProviderSubject: "20", Login: "leaver", Name: "leaver"})
	if err != nil {
		t.Fatal(err)
	}
	if again.UserID == me || !again.Created {
		t.Fatalf("re-sign-up reused the deleted account: %+v", again)
	}
	fresh, err := e.users.GetMe(ctx, again.UserID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.EqualFold(fresh.Handle, "leaver") {
		t.Fatal("the deleted account's handle was reused within the erasure window")
	}
}

func TestErasureWaitsThirtyDaysAndRemovesEveryRow(t *testing.T) {
	e := setup(t)
	me := e.learner(t, "30", "eraseme", 0, false)
	// An XP ledger row: the table is append-only except during erasure.
	if _, err := e.pool.Exec(ctx, `INSERT INTO xp_events (user_id, event_key, delta, reason) VALUES ($1, 'adjustment:test', 5, 'adjustment')`, me); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.RequestDeletion(ctx, me, "eraseme"); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	if n, err := e.svc.EraseDue(ctx, now, 10); err != nil || n != 0 {
		t.Fatalf("erased %d before the delay (%v)", n, err)
	}
	if n, err := e.svc.EraseDue(ctx, now.Add(account.ErasureDelay+time.Minute), 10); err != nil || n != 1 {
		t.Fatalf("erased %d after the delay (%v)", n, err)
	}
	for _, table := range []string{"users WHERE id", "user_settings WHERE user_id", "user_totals WHERE user_id", "xp_events WHERE user_id"} {
		var count int
		if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM `+table+` = $1`, me).Scan(&count); err != nil || count != 0 {
			t.Fatalf("%s: %d rows left (%v)", table, count, err)
		}
	}
	var actorless int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE entity_id = $1 AND actor_user_id IS NULL`, me.String()).Scan(&actorless); err != nil || actorless != 2 {
		t.Fatalf("audit entries without the erased actor = %d (%v)", actorless, err)
	}
	// Erasure doesn't loosen the ledger for anyone else.
	other := e.learner(t, "31", "stays", 0, false)
	if _, err := e.pool.Exec(ctx, `INSERT INTO xp_events (user_id, event_key, delta, reason) VALUES ($1, 'adjustment:test', 5, 'adjustment')`, other); err != nil {
		t.Fatal(err)
	}
	if _, err := e.pool.Exec(ctx, `DELETE FROM xp_events WHERE user_id = $1`, other); err == nil {
		t.Fatal("xp_events accepted a delete outside erasure")
	}
}
