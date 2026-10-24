//go:build integration

package redis

import (
	"context"
	"testing"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testenv"
)

func TestOpenConnects(t *testing.T) {
	ctx := context.Background()
	client, err := Open(ctx, testenv.Require(t, "TEST_REDIS_URL"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = client.Close() }()

	if err := client.Set(ctx, "kubedojo:test:ping", "pong", 0).Err(); err != nil {
		t.Fatal(err)
	}
	got, err := client.GetDel(ctx, "kubedojo:test:ping").Result()
	if err != nil || got != "pong" {
		t.Fatalf("GetDel = %q, %v", got, err)
	}
}
