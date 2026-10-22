// Package labs owns lab sessions. Every write to a lab goes through Store.Mutate, which locks
// the session row (the locking root), checks ownership, and commits state, version bump and
// outbox notification together (ADR-0004).
package labs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/database"
)

// ErrNotFound covers both missing labs and labs owned by someone else (threat T5).
var ErrNotFound = errors.New("lab not found")

// ErrNotActive means the lab is completed, abandoned or expired.
var ErrNotActive = errors.New("lab is not active")

// Store persists lab sessions.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore returns a Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Change is returned by a mutation to say whether lab state changed.
type Change struct {
	StateChanged bool
	// EventCount is reported in the state.changed notification.
	EventCount int
}

// Result of a committed mutation.
type Result struct {
	Session dbgen.LabSession
	Version int64
}

// MutateFunc runs inside the lab's transaction with the session row locked. It must only
// write through q, and must be safe to re-run if the transaction is retried.
type MutateFunc func(ctx context.Context, q *dbgen.Queries, s dbgen.LabSession) (Change, error)

// Mutate locks the user's active lab, runs fn, and if fn changed state, increments the lab
// version and appends a lab.state_changed outbox event — all in one transaction. Any error
// rolls everything back.
func (st *Store) Mutate(ctx context.Context, userID, labID uuid.UUID, fn MutateFunc) (Result, error) {
	var res Result
	err := database.WithTx(ctx, st.pool, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		s, err := q.LockLabSession(ctx, dbgen.LockLabSessionParams{ID: labID, UserID: userID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("lock lab: %w", err)
		}
		if s.State != "active" {
			return ErrNotActive
		}
		change, err := fn(ctx, q, s)
		if err != nil {
			return err
		}
		res = Result{Session: s, Version: s.Version}
		if !change.StateChanged {
			return nil
		}
		version, err := q.BumpLabVersion(ctx, s.ID)
		if err != nil {
			return fmt.Errorf("bump lab version: %w", err)
		}
		payload, err := json.Marshal(map[string]any{"labId": s.ID, "snapshotVersion": version, "eventCount": change.EventCount})
		if err != nil {
			return err
		}
		if _, err := q.InsertOutboxEvent(ctx, dbgen.InsertOutboxEventParams{
			EventType: "lab.state_changed", AggregateType: "lab_session", AggregateID: s.ID, Payload: payload,
		}); err != nil {
			return fmt.Errorf("append outbox event: %w", err)
		}
		res.Version = version
		return nil
	})
	return res, err
}

// Get returns the user's lab (ErrNotFound if missing or owned by someone else).
func (st *Store) Get(ctx context.Context, userID, labID uuid.UUID) (dbgen.LabSession, error) {
	s, err := dbgen.New(st.pool).GetLabSession(ctx, dbgen.GetLabSessionParams{ID: labID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return s, ErrNotFound
	}
	return s, err
}
