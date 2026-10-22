//go:build integration

package seed_test

import (
	"context"
	"testing"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testdb"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/seed"
)

func TestSeedIsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := testdb.New(t)
	counts := func() [3]int {
		var c [3]int
		for i, table := range []string{"courses", "lessons", "achievement_definitions"} {
			if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&c[i]); err != nil {
				t.Fatal(err)
			}
		}
		return c
	}
	for run := 1; run <= 2; run++ {
		c, err := seed.Run(ctx, pool)
		if err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		if c.Courses != 6 || c.Lessons != 30 || c.Achievements != 9 {
			t.Fatalf("run %d wrote %+v", run, c)
		}
		if got := counts(); got != [3]int{6, 30, 9} {
			t.Fatalf("run %d table counts %v", run, got)
		}
	}
	var published int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM courses WHERE published").Scan(&published)
	if published != 0 {
		t.Fatal("seeded courses must stay unpublished until content is imported")
	}
	var order []string
	rows, _ := pool.Query(ctx, "SELECT slug FROM courses ORDER BY display_order")
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		order = append(order, s)
	}
	if len(order) != 6 || order[0] != "fundamentals" || order[5] != "troubleshooting" {
		t.Fatalf("course order %v", order)
	}
}
