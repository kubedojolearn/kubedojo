//go:build integration

package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/redis"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testenv"
)

func TestRedisFixedWindow(t *testing.T) {
	ctx := context.Background()
	client, err := redis.Open(ctx, testenv.Require(t, "TEST_REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()
	l := NewRedis(client)
	key := "test:" + uuid.NewString()

	for i := 1; i <= 3; i++ {
		d, err := l.Allow(ctx, key, 3, 400*time.Millisecond)
		if err != nil || !d.Allowed || d.Remaining != 3-i {
			t.Fatalf("event %d: %+v %v", i, d, err)
		}
	}
	d, err := l.Allow(ctx, key, 3, 400*time.Millisecond)
	if err != nil || d.Allowed || d.RetryAfter <= 0 || d.RetryAfter > 400*time.Millisecond {
		t.Fatalf("over limit: %+v %v", d, err)
	}
	ttl, _ := client.PTTL(ctx, "rl:"+key).Result()
	if ttl <= 0 {
		t.Fatalf("key must expire, ttl %v", ttl)
	}

	time.Sleep(450 * time.Millisecond)
	if d, err := l.Allow(ctx, key, 3, 400*time.Millisecond); err != nil || !d.Allowed {
		t.Fatalf("new window: %+v %v", d, err)
	}
}
