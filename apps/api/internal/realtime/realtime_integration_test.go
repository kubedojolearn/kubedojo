//go:build integration

package realtime_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/redis"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testdb"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testenv"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/realtime"
)

var ctx = context.Background()

func TestTicketsAreOneUse(t *testing.T) {
	rdb, err := redis.Open(ctx, testenv.Require(t, "TEST_REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rdb.Close() }()
	tickets := realtime.NewTickets(rdb)
	user, lab := uuid.New(), uuid.New()
	tk, err := tickets.Issue(ctx, user, lab)
	if err != nil {
		t.Fatal(err)
	}
	if len(tk.Ticket) < 43 || tk.URL != "/api/v1/ws/labs/"+lab.String() || tk.ProtocolVersion != 1 || time.Until(tk.ExpiresAt) > realtime.TicketTTL {
		t.Fatalf("ticket %+v", tk)
	}
	// Only a hash is stored.
	if n, _ := rdb.Exists(ctx, "ws-ticket:"+tk.Ticket).Result(); n != 0 {
		t.Fatal("the raw ticket is a Redis key")
	}
	u, l, err := tickets.Redeem(ctx, tk.Ticket)
	if err != nil || u != user || l != lab {
		t.Fatalf("redeem: %v %s %s", err, u, l)
	}
	if _, _, err := tickets.Redeem(ctx, tk.Ticket); !errors.Is(err, realtime.ErrBadTicket) {
		t.Fatalf("second redeem: %v", err)
	}
	if ttl, _ := rdb.TTL(ctx, "ws-ticket:x").Result(); ttl > 0 {
		t.Fatal("unexpected key")
	}
}

func TestOutboxRelayPublishesStateChanges(t *testing.T) {
	pool := testdb.New(t)
	rdb, err := redis.Open(ctx, testenv.Require(t, "TEST_REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rdb.Close() }()

	lab := uuid.New()
	sub := rdb.Subscribe(ctx, realtime.Channel(lab))
	defer func() { _ = sub.Close() }()
	if _, err := sub.Receive(ctx); err != nil { // subscription confirmed
		t.Fatal(err)
	}

	payload, _ := json.Marshal(map[string]any{"labId": lab, "snapshotVersion": 5, "eventCount": 2})
	if _, err := pool.Exec(ctx, `INSERT INTO outbox_events (event_type, aggregate_type, aggregate_id, payload) VALUES
		('lab.state_changed', 'lab_session', $1, $2), ('lab.other_thing', 'lab_session', $1, '{}')`, lab, payload); err != nil {
		t.Fatal(err)
	}
	n, err := realtime.RelayOutbox(ctx, pool, rdb)
	if err != nil || n != 2 {
		t.Fatalf("relay: %d %v", n, err)
	}
	msg, err := sub.ReceiveMessage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var got realtime.StateChanged
	if err := json.Unmarshal([]byte(msg.Payload), &got); err != nil || got.SnapshotVersion != 5 || got.EventCount != 2 {
		t.Fatalf("message %q: %v", msg.Payload, err)
	}
	var unpublished int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE published_at IS NULL`).Scan(&unpublished)
	if unpublished != 0 {
		t.Fatalf("%d events left unpublished", unpublished)
	}
	if n, err := realtime.RelayOutbox(ctx, pool, rdb); err != nil || n != 0 {
		t.Fatalf("second pass: %d %v", n, err)
	}
}
