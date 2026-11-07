//go:build integration

package database

import (
	"context"
	"testing"
	"time"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testenv"
)

func TestAdvisoryLockSerializesHolders(t *testing.T) {
	ctx := context.Background()
	pool, err := Open(ctx, testenv.Require(t, "TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer pool.Close()
	const key int64 = 0x7465_7374_6c6f_636b // a test-only key, never the bootstrap one

	unlock, err := AdvisoryLock(ctx, pool, key)
	if err != nil {
		t.Fatalf("first AdvisoryLock: %v", err)
	}

	acquired := make(chan func(), 1)
	go func() {
		second, err := AdvisoryLock(ctx, pool, key)
		if err != nil {
			t.Errorf("second AdvisoryLock: %v", err)
			close(acquired)
			return
		}
		acquired <- second
	}()

	select {
	case <-acquired:
		t.Fatal("second holder got the lock while the first still held it")
	case <-time.After(300 * time.Millisecond):
	}

	unlock()
	select {
	case second, ok := <-acquired:
		if !ok {
			t.FailNow()
		}
		second()
	case <-time.After(5 * time.Second):
		t.Fatal("second holder did not get the lock after the first released it")
	}
}

func TestAdvisoryLockGivesUpWhenContextEnds(t *testing.T) {
	ctx := context.Background()
	pool, err := Open(ctx, testenv.Require(t, "TEST_DATABASE_URL"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer pool.Close()
	const key int64 = 0x7465_7374_6c6f_636c

	unlock, err := AdvisoryLock(ctx, pool, key)
	if err != nil {
		t.Fatalf("AdvisoryLock: %v", err)
	}
	defer unlock()

	waitCtx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	if _, err := AdvisoryLock(waitCtx, pool, key); err == nil {
		t.Fatal("expected an error while another session holds the lock")
	}
}
