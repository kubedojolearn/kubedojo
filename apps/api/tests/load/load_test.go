//go:build integration

// Package load verifies rate limits, WebSocket ticket concurrency, and per-lab resource budgets
// under concurrent load (docs/security/threat-model.md, T14).
package load

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/kubectl"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/ratelimit"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/redis"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testenv"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/realtime"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

func TestRateLimitUnderConcurrentLoad(t *testing.T) {
	ctx := context.Background()
	rdb, err := redis.Open(ctx, testenv.Require(t, "REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rdb.Close() }()

	limiter := ratelimit.NewRedis(rdb)
	key := "load-test:" + uuid.NewString()
	const (
		limit   = 25
		workers = 10
		perW    = 6 // 60 total requests against a 25-request window
	)

	var allowed, denied atomic.Int32
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perW {
				dec, err := limiter.Allow(ctx, key, limit, time.Minute)
				if err != nil {
					t.Errorf("limiter error: %v", err)
					return
				}
				if dec.Allowed {
					allowed.Add(1)
				} else {
					if dec.RetryAfter <= 0 {
						t.Errorf("denied without positive RetryAfter: %+v", dec)
					}
					denied.Add(1)
				}
			}
		}()
	}
	wg.Wait()

	if got := allowed.Load(); got != limit {
		t.Fatalf("allowed=%d, want exactly %d (denied=%d)", got, limit, denied.Load())
	}
	if got := denied.Load(); got != workers*perW-limit {
		t.Fatalf("denied=%d, want %d", got, workers*perW-limit)
	}
}

func TestWebSocketTicketSingleUseUnderRace(t *testing.T) {
	ctx := context.Background()
	rdb, err := redis.Open(ctx, testenv.Require(t, "REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rdb.Close() }()

	tickets := realtime.NewTickets(rdb)
	userID, labID := uuid.New(), uuid.New()
	tok, err := tickets.Issue(ctx, userID, labID)
	if err != nil {
		t.Fatalf("issue ticket: %v", err)
	}

	const racers = 20
	var wins atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			gotUser, gotLab, err := tickets.Redeem(ctx, tok.Ticket)
			if errors.Is(err, realtime.ErrBadTicket) {
				return
			}
			if err != nil {
				t.Errorf("redeem error: %v", err)
				return
			}
			if gotUser != userID || gotLab != labID {
				t.Errorf("redeem returned (%s, %s), want (%s, %s)", gotUser, gotLab, userID, labID)
				return
			}
			wins.Add(1)
		}()
	}
	close(start)
	wg.Wait()

	if got := wins.Load(); got != 1 {
		t.Fatalf("single-use ticket redeemed %d times under race, want 1", got)
	}
}

func TestPerLabResourceBudgetsHoldUnderLoad(t *testing.T) {
	st := simulator.NewState(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 7, simulator.ImageCatalog{})
	env := kubectl.Env{State: st, Files: map[string]string{}}
	if res := kubectl.Execute("kubectl create namespace default", env); res.ExitCode != 0 {
		t.Fatalf("create default namespace: %s", res.Stderr)
	}

	// Attempt to create more ConfigMaps than st.Limits.MaxObjects in one lab.
	attempts := st.Limits.MaxObjects + 50
	var created, rejected int
	for i := range attempts {
		res := kubectl.Execute(fmt.Sprintf("kubectl create configmap cm-%d --from-literal=k=v", i), env)
		if res.Err != nil {
			t.Fatalf("unexpected internal error at %d: %v", i, res.Err)
		}
		if res.ExitCode == 0 {
			created++
		} else {
			rejected++
		}
	}
	if created == 0 || rejected == 0 {
		t.Fatalf("expected budget cap to engage: created=%d rejected=%d", created, rejected)
	}
	if total := st.Count(); total > st.Limits.MaxObjects {
		t.Fatalf("simulator exceeded MaxObjects: %d > %d", total, st.Limits.MaxObjects)
	}
}
