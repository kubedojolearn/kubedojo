package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// BootstrapLockKey serializes `api bootstrap` (migrations, seed, content import) across processes.
const BootstrapLockKey int64 = 0x6b75_6265_646f_6a6f // "kubedojo"

// AdvisoryLock blocks until it holds the session-level Postgres advisory lock key, on a connection
// reserved for the holder, and returns the function that releases both. It gives up when ctx ends.
func AdvisoryLock(ctx context.Context, pool *pgxpool.Pool, key int64) (unlock func(), err error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire connection for advisory lock: %w", err)
	}
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", key); err != nil {
		conn.Release()
		return nil, fmt.Errorf("take advisory lock: %w", err)
	}
	return func() {
		// The caller's context may already be done; unlocking must still reach the server.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", key); err != nil {
			// The lock is tied to the session: closing the connection releases it.
			_ = conn.Conn().Close(ctx)
		}
		conn.Release()
	}, nil
}
