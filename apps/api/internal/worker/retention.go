package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/database"
)

// Retention windows (docs/operations/database.md and docs/architecture/requirements.md).
const (
	EndedLabContentRetention = 30 * 24 * time.Hour
	CommandHistoryRetention  = 90 * 24 * time.Hour
	PublishedOutboxRetention = 7 * 24 * time.Hour
	ActivityEventsRetention  = 180 * 24 * time.Hour
	// AuditLogRetention is slightly over 365 days to stay safely past PostgreSQL's `interval '1 year'`
	// trigger check across leap years.
	AuditLogRetention = 366 * 24 * time.Hour
)

// PurgeEndedLabContent deletes simulated resources, files, events, commands and answers for
// ended labs older than EndedLabContentRetention. The lab_sessions row and challenge_attempts
// are preserved. Returns the number of lab sessions purged.
func PurgeEndedLabContent(ctx context.Context, pool *pgxpool.Pool, now time.Time, batch int32) (int, error) {
	cutoff := now.Add(-EndedLabContentRetention)
	var count int
	err := database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		ids, err := q.ListEndedLabsForPurge(ctx, dbgen.ListEndedLabsForPurgeParams{
			Before:     cutoff,
			BatchLimit: batch,
		})
		if err != nil {
			return fmt.Errorf("list ended labs for purge: %w", err)
		}
		if len(ids) == 0 {
			count = 0
			return nil
		}
		if _, err := q.PurgeLabResources(ctx, ids); err != nil {
			return fmt.Errorf("purge lab resources: %w", err)
		}
		if _, err := q.PurgeLabFiles(ctx, ids); err != nil {
			return fmt.Errorf("purge lab files: %w", err)
		}
		if _, err := q.PurgeLabEvents(ctx, ids); err != nil {
			return fmt.Errorf("purge lab events: %w", err)
		}
		if _, err := q.PurgeLabCommands(ctx, ids); err != nil {
			return fmt.Errorf("purge lab commands: %w", err)
		}
		if _, err := q.PurgeLabAnswers(ctx, ids); err != nil {
			return fmt.Errorf("purge lab answers: %w", err)
		}
		count = len(ids)
		return nil
	})
	return count, err
}

// PruneCommandHistory deletes command_executions older than CommandHistoryRetention.
func PruneCommandHistory(ctx context.Context, pool *pgxpool.Pool, now time.Time, batch int32) (int64, error) {
	n, err := dbgen.New(pool).PruneCommandExecutions(ctx, dbgen.PruneCommandExecutionsParams{
		Before:     now.Add(-CommandHistoryRetention),
		BatchLimit: batch,
	})
	if err != nil {
		return 0, fmt.Errorf("prune command history: %w", err)
	}
	return n, nil
}

// PrunePublishedOutbox deletes published outbox_events older than PublishedOutboxRetention.
func PrunePublishedOutbox(ctx context.Context, pool *pgxpool.Pool, now time.Time, batch int32) (int64, error) {
	n, err := dbgen.New(pool).PrunePublishedOutbox(ctx, dbgen.PrunePublishedOutboxParams{
		Before:     now.Add(-PublishedOutboxRetention),
		BatchLimit: batch,
	})
	if err != nil {
		return 0, fmt.Errorf("prune outbox: %w", err)
	}
	return n, nil
}

// PruneActivityEvents deletes activity_events older than ActivityEventsRetention (180 days).
func PruneActivityEvents(ctx context.Context, pool *pgxpool.Pool, now time.Time, batch int32) (int64, error) {
	n, err := dbgen.New(pool).PruneActivityEvents(ctx, dbgen.PruneActivityEventsParams{
		Before:     now.Add(-ActivityEventsRetention),
		BatchLimit: batch,
	})
	if err != nil {
		return 0, fmt.Errorf("prune activity events: %w", err)
	}
	return n, nil
}

// PruneAuditLogs deletes audit_logs older than AuditLogRetention inside a transaction that sets
// kubedojo.audit_retention = 'on' (enforced by the audit_logs_append_only trigger).
func PruneAuditLogs(ctx context.Context, pool *pgxpool.Pool, now time.Time, batch int32) (int64, error) {
	var deleted int64
	err := database.WithTx(ctx, pool, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		if err := q.EnableAuditRetention(ctx); err != nil {
			return fmt.Errorf("enable audit retention: %w", err)
		}
		n, err := q.PruneAuditLogs(ctx, dbgen.PruneAuditLogsParams{
			Before:     now.Add(-AuditLogRetention),
			BatchLimit: batch,
		})
		if err != nil {
			return fmt.Errorf("prune audit logs: %w", err)
		}
		deleted = n
		return nil
	})
	return deleted, err
}
