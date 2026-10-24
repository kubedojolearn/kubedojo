//go:build integration

package gamification_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/gamification"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/database"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testdb"
)

var ctx = context.Background()

func TestGrantXPIsIdempotent(t *testing.T) {
	pool := testdb.New(t)
	user := testdb.CreateUser(t, pool, "u")
	lesson := uuid.New()
	grant := gamification.Grant{UserID: user, EventKey: gamification.LessonCompletedKey(lesson), Delta: 125, Reason: gamification.ReasonLessonCompleted}

	for i, wantGranted := range []bool{true, false, false} {
		var granted bool
		var total int32
		err := database.WithTx(ctx, pool, func(tx pgx.Tx) error {
			var err error
			granted, total, err = gamification.GrantXP(ctx, dbgen.New(tx), grant)
			return err
		})
		if err != nil || granted != wantGranted || total != 125 {
			t.Fatalf("call %d: granted=%v total=%d err=%v", i, granted, total, err)
		}
	}
}

func TestConcurrentGrantsAwardOnce(t *testing.T) {
	pool := testdb.New(t)
	user := testdb.CreateUser(t, pool, "u")
	grant := gamification.Grant{UserID: user, EventKey: gamification.LessonCompletedKey(uuid.New()), Delta: 100, Reason: gamification.ReasonLessonCompleted}

	var granted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Go(func() {
			err := database.WithTx(ctx, pool, func(tx pgx.Tx) error {
				g, _, err := gamification.GrantXP(ctx, dbgen.New(tx), grant)
				if g {
					granted.Add(1)
				}
				return err
			})
			if err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if granted.Load() != 1 {
		t.Fatalf("granted %d times", granted.Load())
	}
	q := dbgen.New(pool)
	totals, _ := q.GetUserTotals(ctx, user)
	sum, _ := q.SumXPEvents(ctx, user)
	if totals.TotalXp != 100 || sum != 100 {
		t.Fatalf("total %d, ledger sum %d", totals.TotalXp, sum)
	}
}

func TestTotalsMatchLedger(t *testing.T) {
	pool := testdb.New(t)
	user := testdb.CreateUser(t, pool, "u")
	deltas := []int32{100, 25, -10, 50}
	err := database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		for i, d := range deltas {
			if _, _, err := gamification.GrantXP(ctx, dbgen.New(tx), gamification.Grant{
				UserID: user, EventKey: "test:" + string(rune('a'+i)), Delta: d, Reason: gamification.ReasonAdjustment,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	q := dbgen.New(pool)
	totals, _ := q.GetUserTotals(ctx, user)
	sum, _ := q.SumXPEvents(ctx, user)
	if int64(totals.TotalXp) != sum || sum != 165 {
		t.Fatalf("total %d, ledger %d", totals.TotalXp, sum)
	}
}

func TestGrantRollsBackWithTransaction(t *testing.T) {
	pool := testdb.New(t)
	user := testdb.CreateUser(t, pool, "u")
	_ = database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		if _, _, err := gamification.GrantXP(ctx, dbgen.New(tx), gamification.Grant{UserID: user, EventKey: "test:x", Delta: 50, Reason: gamification.ReasonAdjustment}); err != nil {
			return err
		}
		return context.Canceled // e.g. grading failed later in the same transaction
	})
	totals, _ := dbgen.New(pool).GetUserTotals(ctx, user)
	if totals.TotalXp != 0 {
		t.Fatalf("XP survived a rolled-back transaction: %d", totals.TotalXp)
	}
}
