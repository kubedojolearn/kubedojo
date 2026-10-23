package labs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/database"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// Errors returned by Service besides ErrNotFound, ErrNotActive and ErrExpired.
var (
	// ErrChallengeNotFound: the challenge version doesn't exist or isn't published.
	ErrChallengeNotFound = errors.New("challenge version not found")
	// ErrStaleChallenge: the version was replaced by a newer one, or needs another simulator.
	ErrStaleChallenge = errors.New("challenge version is not current")
	// ErrIdempotencyConflict: the key was already used for a different lab.
	ErrIdempotencyConflict = errors.New("idempotency key reused")
)

// Lab mirrors the OpenAPI Lab.
type Lab struct {
	ID                 uuid.UUID  `json:"id"`
	LessonID           string     `json:"lessonId"`
	CourseSlug         string     `json:"courseSlug"`
	LessonSlug         string     `json:"lessonSlug"`
	Title              string     `json:"title"`
	ChallengeVersionID uuid.UUID  `json:"challengeVersionId"`
	State              string     `json:"state"`
	Version            int64      `json:"version"`
	CommandSeq         int64      `json:"commandSeq"`
	SimulatorVersion   string     `json:"simulatorVersion"`
	StartedAt          time.Time  `json:"startedAt"`
	LastActiveAt       time.Time  `json:"lastActiveAt"`
	ExpiresAt          time.Time  `json:"expiresAt"`
	EndedAt            *time.Time `json:"endedAt"`
	HintsUsed          int16      `json:"hintsUsed"`
}

// Snapshot mirrors the OpenAPI Snapshot.
type Snapshot struct {
	LabID            uuid.UUID         `json:"labId"`
	Version          int64             `json:"version"`
	SimulatorVersion string            `json:"simulatorVersion"`
	SimTime          time.Time         `json:"simTime"`
	CurrentNamespace string            `json:"currentNamespace"`
	Resources        []json.RawMessage `json:"resources"`
}

// InvolvedObject mirrors SimEvent.involvedObject.
type InvolvedObject struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
}

// SimEvent mirrors the OpenAPI SimEvent.
type SimEvent struct {
	Seq            int64          `json:"seq"`
	Version        int64          `json:"version"`
	Type           string         `json:"type"`
	Reason         string         `json:"reason"`
	Message        string         `json:"message"`
	InvolvedObject InvolvedObject `json:"involvedObject"`
	OccurredAt     time.Time      `json:"occurredAt"`
}

// ListCursor is the keyset position in the lab list.
type ListCursor struct {
	LastActiveAt time.Time `json:"a"`
	ID           uuid.UUID `json:"i"`
}

// Service implements the lab lifecycle on top of Store.
type Service struct {
	pool     *pgxpool.Pool
	store    *Store
	fixtures *fixtureCache
	now      func() time.Time
}

// NewService returns a Service.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, store: NewStore(pool), fixtures: &fixtureCache{}, now: time.Now}
}

// Store exposes the underlying store (commands mutate through it).
func (sv *Service) Store() *Store { return sv.store }

func viewToLab(v dbgen.GetLabViewRow, now time.Time) Lab {
	l := Lab{
		ID: v.ID, LessonID: v.CourseSlug + "/" + v.LessonSlug, CourseSlug: v.CourseSlug, LessonSlug: v.LessonSlug,
		Title: v.LessonTitle, ChallengeVersionID: v.ChallengeVersionID, State: v.State, Version: v.Version,
		CommandSeq: v.CommandSeq, SimulatorVersion: v.SimulatorVersion, StartedAt: v.StartedAt,
		LastActiveAt: v.LastActiveAt, ExpiresAt: v.ExpiresAt, EndedAt: v.EndedAt, HintsUsed: v.HintsUsed,
	}
	// The expiry sweep runs every minute; until it does, report what the next write will find.
	if l.State == "active" && !now.Before(l.ExpiresAt) {
		l.State, l.EndedAt = "expired", &l.ExpiresAt
	}
	return l
}

func (sv *Service) view(ctx context.Context, q *dbgen.Queries, userID, labID uuid.UUID) (Lab, error) {
	v, err := q.GetLabView(ctx, dbgen.GetLabViewParams{ID: labID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return Lab{}, ErrNotFound
	}
	if err != nil {
		return Lab{}, err
	}
	return viewToLab(v, sv.now()), nil
}

// Create starts a lab from a challenge version's fixture, or returns the learner's active lab
// for that lesson (created=false).
func (sv *Service) Create(ctx context.Context, userID, challengeVersionID uuid.UUID) (lab Lab, created bool, err error) {
	for attempt := 0; attempt < 2; attempt++ {
		lab, created, err = sv.create(ctx, userID, challengeVersionID)
		// Two concurrent starts for one lesson: the loser retries and returns the winner's lab.
		if !database.IsCode(err, database.CodeUniqueViolation) {
			return lab, created, err
		}
	}
	return lab, created, err
}

func (sv *Service) create(ctx context.Context, userID, challengeVersionID uuid.UUID) (Lab, bool, error) {
	var lab Lab
	created := false
	err := database.WithTx(ctx, sv.pool, func(tx pgx.Tx) error {
		q := dbgen.New(tx)
		cv, err := q.GetLaunchableChallengeVersion(ctx, challengeVersionID)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrChallengeNotFound
		}
		if err != nil {
			return fmt.Errorf("challenge version: %w", err)
		}

		existing, err := q.GetActiveLabForLesson(ctx, dbgen.GetActiveLabForLessonParams{UserID: userID, LessonID: cv.LessonID})
		switch {
		case err == nil && endReason(existing, sv.now()) == "":
			lab, err = sv.view(ctx, q, userID, existing.ID)
			return err
		case err == nil:
			// The active lab has lapsed: end it so a fresh one can start.
			reason := endReason(existing, sv.now())
			if err := q.EndLab(ctx, dbgen.EndLabParams{ID: existing.ID, State: "expired", StateReason: &reason}); err != nil {
				return err
			}
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}

		if !cv.IsCurrent || cv.SimulatorVersion != simulator.Version {
			return ErrStaleChallenge
		}
		f, err := sv.fixtures.get(ctx, q, cv.ID)
		if err != nil {
			return err
		}
		seed, err := newSeed()
		if err != nil {
			return err
		}
		state, err := simulator.Load(f, toUint(seed))
		if err != nil {
			// Fixtures are validated at import; failing here is a content or simulator bug.
			return fmt.Errorf("load fixture of challenge version %s: %w", cv.ID, err)
		}
		s, err := q.CreateLabSession(ctx, dbgen.CreateLabSessionParams{
			UserID: userID, LessonID: cv.LessonID, ChallengeVersionID: cv.ID, SimulatorVersion: simulator.Version,
			Seed: seed, SimTime: state.Now, ExpiresAt: sv.now().Add(IdleTTL),
		})
		if err != nil {
			return err
		}
		if _, err := persist(ctx, q, s, state, s.Version); err != nil {
			return err
		}
		if err := q.RecordLabStarted(ctx, dbgen.RecordLabStartedParams{UserID: userID, LessonID: cv.LessonID}); err != nil {
			return fmt.Errorf("record attempt: %w", err)
		}
		optOut, err := q.GetAnalyticsOptOut(ctx, userID)
		if err != nil {
			return err
		}
		if !optOut {
			lessonID := cv.LessonID
			if err := q.InsertActivity(ctx, dbgen.InsertActivityParams{UserID: userID, Type: "lab-started", LessonID: &lessonID}); err != nil {
				return err
			}
		}
		created = true
		lab, err = sv.view(ctx, q, userID, s.ID)
		return err
	})
	return lab, created, err
}

// Get returns the learner's lab.
func (sv *Service) Get(ctx context.Context, userID, labID uuid.UUID) (Lab, error) {
	return sv.view(ctx, dbgen.New(sv.pool), userID, labID)
}

// List returns a page of the learner's labs, most recently active first.
func (sv *Service) List(ctx context.Context, userID uuid.UUID, state *string, after *ListCursor, limit int32) ([]Lab, *ListCursor, error) {
	params := dbgen.ListUserLabViewsParams{UserID: userID, State: state, PageLimit: limit + 1}
	if after != nil {
		params.AfterActiveAt, params.AfterID = &after.LastActiveAt, &after.ID
	}
	rows, err := dbgen.New(sv.pool).ListUserLabViews(ctx, params)
	if err != nil {
		return nil, nil, err
	}
	var next *ListCursor
	if len(rows) > int(limit) {
		last := rows[limit-1]
		next = &ListCursor{LastActiveAt: last.LastActiveAt, ID: last.ID}
		rows = rows[:limit]
	}
	now := sv.now()
	out := make([]Lab, 0, len(rows))
	for _, r := range rows {
		out = append(out, viewToLab(dbgen.GetLabViewRow(r), now))
	}
	return out, next, nil
}

// Resume marks the lab used now, extending its expiry.
func (sv *Service) Resume(ctx context.Context, userID, labID uuid.UUID) (Lab, error) {
	if _, err := sv.store.Mutate(ctx, userID, labID, func(context.Context, *dbgen.Queries, dbgen.LabSession) (Change, error) {
		return Change{}, nil
	}); err != nil {
		return Lab{}, err
	}
	return sv.Get(ctx, userID, labID)
}

// Idempotent operations (Idempotency-Key header).
const (
	OpReset  = "lab.reset"
	OpFinish = "lab.finish"
)

// replay returns the stored result of an earlier request with this key, if there was one.
func replay(ctx context.Context, q *dbgen.Queries, userID uuid.UUID, op, key string, labID uuid.UUID) (*Lab, error) {
	rec, err := q.GetIdempotencyRecord(ctx, dbgen.GetIdempotencyRecordParams{UserID: userID, Operation: op, Key: key})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if rec.TargetID != labID {
		return nil, ErrIdempotencyConflict
	}
	var lab Lab
	if err := json.Unmarshal(rec.Response, &lab); err != nil {
		return nil, fmt.Errorf("stored idempotent response: %w", err)
	}
	return &lab, nil
}

// idempotent runs a lab mutation at most once per (user, op, key): fn performs the change, and
// the resulting lab is stored with the key in the same transaction.
func (sv *Service) idempotent(ctx context.Context, userID, labID uuid.UUID, op, key string, fn func(ctx context.Context, q *dbgen.Queries, s dbgen.LabSession) (Change, error)) (Lab, error) {
	if lab, err := replay(ctx, dbgen.New(sv.pool), userID, op, key, labID); err != nil || lab != nil {
		return derefLab(lab), err
	}
	var result Lab
	_, err := sv.store.Mutate(ctx, userID, labID, func(ctx context.Context, q *dbgen.Queries, s dbgen.LabSession) (Change, error) {
		// A concurrent request with this key may have committed while we waited for the lock.
		if lab, err := replay(ctx, q, userID, op, key, labID); err != nil || lab != nil {
			result = derefLab(lab)
			return Change{}, err
		}
		change, err := fn(ctx, q, s)
		if err != nil {
			return change, err
		}
		change.Then = func(ctx context.Context, q *dbgen.Queries, _ int64) error {
			lab, err := sv.view(ctx, q, userID, labID)
			if err != nil {
				return err
			}
			body, err := json.Marshal(lab)
			if err != nil {
				return err
			}
			result = lab
			err = q.InsertIdempotencyRecord(ctx, dbgen.InsertIdempotencyRecordParams{
				UserID: userID, Operation: op, Key: key, TargetID: labID, StatusCode: http.StatusOK, Response: body,
			})
			if database.IsCode(err, database.CodeUniqueViolation) {
				return ErrIdempotencyConflict // same key, different lab, committed concurrently
			}
			return err
		}
		return change, nil
	})
	if errors.Is(err, ErrNotActive) {
		// A retry of a finish that already committed finds the lab ended: answer from the record.
		if lab, rerr := replay(ctx, dbgen.New(sv.pool), userID, op, key, labID); rerr != nil || lab != nil {
			return derefLab(lab), rerr
		}
	}
	return result, err
}

func derefLab(l *Lab) Lab {
	if l == nil {
		return Lab{}
	}
	return *l
}

// Reset reloads the lab from its challenge version's fixture with the same seed. The version
// and resource versions keep increasing; earlier events are dropped with the old state.
func (sv *Service) Reset(ctx context.Context, userID, labID uuid.UUID, key string) (Lab, error) {
	return sv.idempotent(ctx, userID, labID, OpReset, key, func(ctx context.Context, q *dbgen.Queries, s dbgen.LabSession) (Change, error) {
		f, err := sv.fixtures.get(ctx, q, s.ChallengeVersionID)
		if err != nil {
			return Change{}, err
		}
		state, err := simulator.Load(f, toUint(s.Seed))
		if err != nil {
			return Change{}, fmt.Errorf("reload fixture: %w", err)
		}
		state.Rebase(s.ResourceRevision)
		if err := q.DeleteAllLabResources(ctx, s.ID); err != nil {
			return Change{}, err
		}
		if err := q.DeleteLabEvents(ctx, s.ID); err != nil {
			return Change{}, err
		}
		p, err := persist(ctx, q, s, state, s.Version+1)
		if err != nil {
			return Change{}, err
		}
		return Change{StateChanged: true, EventCount: p.Events}, nil
	})
}

// Finish ends the lab without completing it (state abandoned).
func (sv *Service) Finish(ctx context.Context, userID, labID uuid.UUID, key string) (Lab, error) {
	return sv.idempotent(ctx, userID, labID, OpFinish, key, func(ctx context.Context, q *dbgen.Queries, s dbgen.LabSession) (Change, error) {
		reason := ReasonFinished
		return Change{}, q.EndLab(ctx, dbgen.EndLabParams{ID: s.ID, State: "abandoned", StateReason: &reason})
	})
}

// readTx runs fn in a read-only repeatable-read transaction, so a lab's row and its resources
// are read at one version.
func (sv *Service) readTx(ctx context.Context, fn func(q *dbgen.Queries) error) error {
	tx, err := sv.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(dbgen.New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Snapshot returns every resource of the lab at its current version.
func (sv *Service) Snapshot(ctx context.Context, userID, labID uuid.UUID) (Snapshot, error) {
	var snap Snapshot
	err := sv.readTx(ctx, func(q *dbgen.Queries) error {
		s, err := q.GetLabSession(ctx, dbgen.GetLabSessionParams{ID: labID, UserID: userID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		// Resources are decoded only, never reconciled: reading doesn't move the clock (ADR-0006).
		state, err := LoadState(ctx, q, s, simulator.Fixture{})
		if err != nil {
			return err
		}
		snap = Snapshot{
			LabID: s.ID, Version: s.Version, SimulatorVersion: s.SimulatorVersion, SimTime: s.SimTime,
			CurrentNamespace: s.CurrentNamespace, Resources: []json.RawMessage{},
		}
		for _, o := range state.All() {
			raw, err := simulator.MarshalForOutput(o)
			if err != nil {
				return err
			}
			snap.Resources = append(snap.Resources, raw)
		}
		return nil
	})
	return snap, err
}

// Events returns the lab's events after a sequence number, oldest first.
func (sv *Service) Events(ctx context.Context, userID, labID uuid.UUID, afterSeq int64, limit int32) ([]SimEvent, *int64, error) {
	q := dbgen.New(sv.pool)
	if _, err := q.GetLabSession(ctx, dbgen.GetLabSessionParams{ID: labID, UserID: userID}); errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	} else if err != nil {
		return nil, nil, err
	}
	rows, err := q.ListLabEvents(ctx, dbgen.ListLabEventsParams{SessionID: labID, AfterSeq: afterSeq, PageLimit: limit + 1})
	if err != nil {
		return nil, nil, err
	}
	var next *int64
	if len(rows) > int(limit) {
		seq := rows[limit-1].Seq
		next = &seq
		rows = rows[:limit]
	}
	out := make([]SimEvent, 0, len(rows))
	for _, r := range rows {
		out = append(out, SimEvent{
			Seq: r.Seq, Version: r.Version, Type: r.Type, Reason: r.Reason, Message: r.Message, OccurredAt: r.OccurredAt,
			InvolvedObject: InvolvedObject{Kind: r.InvolvedKind, Namespace: r.InvolvedNamespace, Name: r.InvolvedName},
		})
	}
	return out, next, nil
}

// ExpireIdle ends active labs past their idle expiry, in batches, and returns how many.
func ExpireIdle(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	const batch = 500
	var total int64
	for {
		n, err := dbgen.New(pool).ExpireLabs(ctx, batch)
		total += n
		if err != nil || n < batch {
			return total, err
		}
	}
}

// IdempotencyRetention is how long idempotent results are kept for retries.
const IdempotencyRetention = 7 * 24 * time.Hour

// PruneIdempotency deletes idempotency records older than IdempotencyRetention.
func PruneIdempotency(ctx context.Context, pool *pgxpool.Pool, now time.Time) (int64, error) {
	return dbgen.New(pool).DeleteOldIdempotencyRecords(ctx, now.Add(-IdempotencyRetention))
}

// Simulate runs fn against the lab's simulated cluster inside the lab's transaction.
//
// A read (mutating=false) sees the stored state as is: the clock doesn't move and nothing is
// written (ADR-0006). A mutation advances the clock one Tick before fn, reconciles after it,
// and persists the changes and events with the next lab version. If fn returns an error,
// nothing is written.
func (sv *Service) Simulate(ctx context.Context, userID, labID uuid.UUID, mutating bool, fn func(*simulator.State) error) (Result, error) {
	return sv.store.Mutate(ctx, userID, labID, func(ctx context.Context, q *dbgen.Queries, s dbgen.LabSession) (Change, error) {
		f, err := sv.fixtures.get(ctx, q, s.ChallengeVersionID)
		if err != nil {
			return Change{}, err
		}
		state, err := LoadState(ctx, q, s, f)
		if err != nil {
			return Change{}, err
		}
		if !mutating {
			return Change{}, fn(state)
		}
		state.Advance(simulator.Tick)
		if err := fn(state); err != nil {
			return Change{}, err
		}
		if _, err := simulator.Reconcile(state); err != nil {
			return Change{}, err
		}
		p, err := persist(ctx, q, s, state, s.Version+1)
		if err != nil {
			return Change{}, err
		}
		// The clock moved even if no object changed, so the version moves too.
		return Change{StateChanged: true, EventCount: p.Events}, nil
	})
}
