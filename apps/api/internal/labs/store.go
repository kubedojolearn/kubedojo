// Package labs owns lab sessions. Every write to a lab goes through Store.Mutate, which locks
// the session row (the locking root), checks ownership, and commits state, version bump and
// outbox notification together (ADR-0004).
package labs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/database"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// IdleTTL is how long a lab stays active without activity (architecture overview §3).
const IdleTTL = 14 * 24 * time.Hour

// Reasons a lab ended, stored in lab_sessions.state_reason.
const (
	ReasonIdleTimeout       = "IDLE_TIMEOUT"
	ReasonSimulatorUpgraded = "SIMULATOR_UPGRADED"
	ReasonFinished          = "FINISHED_BY_LEARNER"
)

// ErrNotFound covers both missing labs and labs owned by someone else (threat T5).
var ErrNotFound = errors.New("lab not found")

// ErrNotActive means the lab is completed, abandoned or expired.
var ErrNotActive = errors.New("lab is not active")

// ErrExpired means the lab was active but has just been found past its idle expiry, or its
// simulator version is no longer supported. Mutate marks it expired before returning this.
var ErrExpired = errors.New("lab has expired")

// Store persists lab sessions.
type Store struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

// NewStore returns a Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool, now: time.Now} }

// Change is returned by a mutation to say whether lab state changed.
type Change struct {
	StateChanged bool
	// EventCount is reported in the state.changed notification.
	EventCount int
	// Then, if set, runs last in the same transaction with the lab's final version.
	Then func(ctx context.Context, q *dbgen.Queries, version int64) error
}

// endReason says why an active lab can no longer be used, or "" if it can.
func endReason(s dbgen.LabSession, now time.Time) string {
	switch {
	case s.SimulatorVersion != simulator.Version:
		// Old state is never reinterpreted by a newer simulator (architecture overview §3).
		return ReasonSimulatorUpgraded
	case !now.Before(s.ExpiresAt):
		return ReasonIdleTimeout
	}
	return ""
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
// rolls everything back. Every mutation, including a read-only one, counts as activity and
// extends the lab's idle expiry. A lab found past its expiry is marked expired (committed) and
// ErrExpired is returned without running fn.
func (st *Store) Mutate(ctx context.Context, userID, labID uuid.UUID, fn MutateFunc) (Result, error) {
	var res Result
	expired := false
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
		now := st.now()
		if reason := endReason(s, now); reason != "" {
			expired = true
			return q.EndLab(ctx, dbgen.EndLabParams{ID: s.ID, State: "expired", StateReason: &reason})
		}
		change, err := fn(ctx, q, s)
		if err != nil {
			return err
		}
		res = Result{Session: s, Version: s.Version}
		expiresAt := now.Add(IdleTTL)
		if !change.StateChanged {
			if err := q.TouchLab(ctx, dbgen.TouchLabParams{ID: s.ID, ExpiresAt: expiresAt}); err != nil {
				return fmt.Errorf("touch lab: %w", err)
			}
			if change.Then != nil {
				return change.Then(ctx, q, s.Version)
			}
			return nil
		}
		version, err := q.BumpLabVersion(ctx, dbgen.BumpLabVersionParams{ID: s.ID, ExpiresAt: expiresAt})
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
		if change.Then != nil {
			return change.Then(ctx, q, version)
		}
		return nil
	})
	if err == nil && expired {
		return res, ErrExpired
	}
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
