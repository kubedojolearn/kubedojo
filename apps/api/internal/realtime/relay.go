package realtime

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/database"
)

// RelayBatch is how many outbox events one relay pass publishes.
const RelayBatch = 200

// RelayOutbox publishes committed lab.state_changed outbox events to their lab's Redis channel
// and marks them published, in batches, until none are left (ADR-0004). Concurrent workers skip
// each other's rows. A failed publish is recorded and retried on the next pass; a lost
// notification only delays a refresh, because clients also refetch on reconnect and on gaps.
func RelayOutbox(ctx context.Context, pool *pgxpool.Pool, rdb goredis.Cmdable) (int, error) {
	total := 0
	for {
		n, err := relayBatch(ctx, pool, rdb)
		total += n
		if err != nil || n < RelayBatch {
			return total, err
		}
	}
}

func relayBatch(ctx context.Context, pool *pgxpool.Pool, rdb goredis.Cmdable) (int, error) {
	published := 0
	var publishErr error
	err := database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		rows, err := q.ClaimOutboxBatch(ctx, RelayBatch)
		if err != nil {
			return err
		}
		var done []int64
		for _, e := range rows {
			if e.EventType != "lab.state_changed" {
				done = append(done, e.ID) // nothing subscribes to other types yet
				continue
			}
			var n StateChanged
			if err := json.Unmarshal(e.Payload, &n); err != nil {
				done = append(done, e.ID) // unreadable forever; don't block the queue
				continue
			}
			body, _ := json.Marshal(StateChanged{SnapshotVersion: n.SnapshotVersion, EventCount: n.EventCount})
			if err := rdb.Publish(ctx, Channel(e.AggregateID), body).Err(); err != nil {
				// Keep what was published, record the failure (committed), retry next pass.
				publishErr = fmt.Errorf("publish outbox event %d: %w", e.ID, err)
				if err := q.RecordOutboxFailure(ctx, dbgen.RecordOutboxFailureParams{ID: e.ID, LastError: err.Error()}); err != nil {
					return err
				}
				break
			}
			done = append(done, e.ID)
		}
		published = len(done)
		if len(done) == 0 {
			return nil
		}
		return q.MarkOutboxPublished(ctx, done)
	})
	if err != nil {
		return published, err
	}
	if publishErr != nil {
		return published, publishErr
	}
	return published, nil
}
