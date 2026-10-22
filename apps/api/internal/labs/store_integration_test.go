//go:build integration

package labs_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/labs"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testdb"
)

var ctx = context.Background()

func addEvent(ctx context.Context, q *dbgen.Queries, s dbgen.LabSession, reason string) error {
	// Writes inside the mutation use the transaction's queries.
	_, err := q.InsertOutboxEvent(ctx, dbgen.InsertOutboxEventParams{
		EventType: "test.marker", AggregateType: "lab_session", AggregateID: s.ID, Payload: json.RawMessage(`{"r":"` + reason + `"}`),
	})
	return err
}

func TestMutateBumpsVersionAndWritesOutboxAtomically(t *testing.T) {
	pool := testdb.New(t)
	user := testdb.CreateUser(t, pool, "u")
	lab := testdb.CreateLab(t, pool, user, testdb.CreatePublishedLesson(t, pool, "x"))
	st := labs.NewStore(pool)

	res, err := st.Mutate(ctx, user, lab.ID, func(ctx context.Context, q *dbgen.Queries, s dbgen.LabSession) (labs.Change, error) {
		return labs.Change{StateChanged: true, EventCount: 2}, addEvent(ctx, q, s, "ok")
	})
	if err != nil || res.Version != 2 {
		t.Fatalf("%+v %v", res, err)
	}
	var payload json.RawMessage
	if err := pool.QueryRow(ctx, `SELECT payload FROM outbox_events WHERE event_type = 'lab.state_changed' AND aggregate_id = $1`, lab.ID).Scan(&payload); err != nil {
		t.Fatalf("outbox row: %v", err)
	}
	var p struct {
		LabID           uuid.UUID `json:"labId"`
		SnapshotVersion int64     `json:"snapshotVersion"`
		EventCount      int       `json:"eventCount"`
	}
	_ = json.Unmarshal(payload, &p)
	if p.LabID != lab.ID || p.SnapshotVersion != 2 || p.EventCount != 2 {
		t.Fatalf("payload %s", payload)
	}

	// A read-only mutation changes nothing.
	res, err = st.Mutate(ctx, user, lab.ID, func(context.Context, *dbgen.Queries, dbgen.LabSession) (labs.Change, error) {
		return labs.Change{}, nil
	})
	if err != nil || res.Version != 2 {
		t.Fatalf("read-only bumped version: %+v %v", res, err)
	}
}

func TestMutateRollsBackEverythingOnError(t *testing.T) {
	pool := testdb.New(t)
	user := testdb.CreateUser(t, pool, "u")
	lab := testdb.CreateLab(t, pool, user, testdb.CreatePublishedLesson(t, pool, "x"))
	boom := errors.New("simulator bug")

	_, err := labs.NewStore(pool).Mutate(ctx, user, lab.ID, func(ctx context.Context, q *dbgen.Queries, s dbgen.LabSession) (labs.Change, error) {
		if err := addEvent(ctx, q, s, "partial"); err != nil {
			return labs.Change{}, err
		}
		return labs.Change{StateChanged: true}, boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("got %v", err)
	}
	var version int64
	var outbox int
	_ = pool.QueryRow(ctx, `SELECT version FROM lab_sessions WHERE id = $1`, lab.ID).Scan(&version)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events`).Scan(&outbox)
	if version != 1 || outbox != 0 {
		t.Fatalf("half-committed state: version %d, outbox rows %d", version, outbox)
	}
}

func TestOtherUsersLabIsNotFound(t *testing.T) {
	pool := testdb.New(t)
	owner := testdb.CreateUser(t, pool, "owner")
	intruder := testdb.CreateUser(t, pool, "intruder")
	lab := testdb.CreateLab(t, pool, owner, testdb.CreatePublishedLesson(t, pool, "x"))
	st := labs.NewStore(pool)

	called := false
	_, err := st.Mutate(ctx, intruder, lab.ID, func(context.Context, *dbgen.Queries, dbgen.LabSession) (labs.Change, error) {
		called = true
		return labs.Change{StateChanged: true}, nil
	})
	if !errors.Is(err, labs.ErrNotFound) || called {
		t.Fatalf("intruder reached the lab: %v called=%v", err, called)
	}
	if _, err := st.Get(ctx, intruder, lab.ID); !errors.Is(err, labs.ErrNotFound) {
		t.Fatalf("intruder read the lab: %v", err)
	}
	if _, err := st.Get(ctx, owner, uuid.New()); !errors.Is(err, labs.ErrNotFound) {
		t.Fatalf("missing lab: %v", err)
	}
	if _, err := st.Get(ctx, owner, lab.ID); err != nil {
		t.Fatalf("owner: %v", err)
	}
}

func TestEndedLabsRejectMutations(t *testing.T) {
	pool := testdb.New(t)
	user := testdb.CreateUser(t, pool, "u")
	lab := testdb.CreateLab(t, pool, user, testdb.CreatePublishedLesson(t, pool, "x"))
	if _, err := pool.Exec(ctx, `UPDATE lab_sessions SET state = 'expired', ended_at = now() WHERE id = $1`, lab.ID); err != nil {
		t.Fatal(err)
	}
	_, err := labs.NewStore(pool).Mutate(ctx, user, lab.ID, func(context.Context, *dbgen.Queries, dbgen.LabSession) (labs.Change, error) {
		return labs.Change{StateChanged: true}, nil
	})
	if !errors.Is(err, labs.ErrNotActive) {
		t.Fatalf("got %v", err)
	}
}

func TestConcurrentMutationsSerialize(t *testing.T) {
	pool := testdb.New(t)
	user := testdb.CreateUser(t, pool, "u")
	lab := testdb.CreateLab(t, pool, user, testdb.CreatePublishedLesson(t, pool, "x"))
	st := labs.NewStore(pool)

	const workers, each = 8, 10
	seen := make(chan int64, workers*each)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Go(func() {
			for i := 0; i < each; i++ {
				res, err := st.Mutate(ctx, user, lab.ID, func(ctx context.Context, q *dbgen.Queries, s dbgen.LabSession) (labs.Change, error) {
					// Read-modify-write that would lose updates without the row lock.
					_, err := q.InsertOutboxEvent(ctx, dbgen.InsertOutboxEventParams{
						EventType: "test.seen_version", AggregateType: "lab_session", AggregateID: s.ID,
						Payload: json.RawMessage(`{"v":` + jsonInt(s.Version) + `}`),
					})
					return labs.Change{StateChanged: true}, err
				})
				if err != nil {
					t.Error(err)
					return
				}
				seen <- res.Version
			}
		})
	}
	wg.Wait()
	close(seen)

	versions := map[int64]bool{}
	for v := range seen {
		if versions[v] {
			t.Fatalf("version %d returned twice: writes were not serialized", v)
		}
		versions[v] = true
	}
	var final int64
	_ = pool.QueryRow(ctx, `SELECT version FROM lab_sessions WHERE id = $1`, lab.ID).Scan(&final)
	if final != 1+workers*each || len(versions) != workers*each {
		t.Fatalf("final version %d, distinct %d", final, len(versions))
	}
	// Every mutation saw the version left by the previous one.
	var distinctSeen int
	_ = pool.QueryRow(ctx, `SELECT count(DISTINCT payload->>'v') FROM outbox_events WHERE event_type = 'test.seen_version'`).Scan(&distinctSeen)
	if distinctSeen != workers*each {
		t.Fatalf("lost update: %d distinct observed versions", distinctSeen)
	}
}

func jsonInt(v int64) string { b, _ := json.Marshal(v); return string(b) }

func TestListUserLabsPaginates(t *testing.T) {
	pool := testdb.New(t)
	user := testdb.CreateUser(t, pool, "u")
	other := testdb.CreateUser(t, pool, "other")
	q := dbgen.New(pool)
	var created []uuid.UUID
	for i := 0; i < 5; i++ {
		l := testdb.CreatePublishedLesson(t, pool, "lesson-"+string(rune('a'+i)))
		created = append(created, testdb.CreateLab(t, pool, user, l).ID)
		testdb.CreateLab(t, pool, other, l)
	}
	// Ties on last_active_at are broken by id.
	if _, err := pool.Exec(ctx, `UPDATE lab_sessions SET last_active_at = '2026-10-01T00:00:00Z' WHERE user_id = $1`, user); err != nil {
		t.Fatal(err)
	}

	var got []uuid.UUID
	params := dbgen.ListUserLabsParams{UserID: user, PageLimit: 2}
	for page := 0; page < 5; page++ {
		rows, err := q.ListUserLabs(ctx, params)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if r.UserID != user {
				t.Fatal("another user's lab in the list")
			}
			got = append(got, r.ID)
		}
		if len(rows) < 2 {
			break
		}
		last := rows[len(rows)-1]
		params.AfterActiveAt, params.AfterID = &last.LastActiveAt, &last.ID
	}
	if len(got) != len(created) {
		t.Fatalf("paged %d labs, want %d", len(got), len(created))
	}
	unique := map[uuid.UUID]bool{}
	for _, id := range got {
		unique[id] = true
	}
	if len(unique) != len(created) {
		t.Fatal("duplicate labs across pages")
	}

	state := "completed"
	if rows, _ := q.ListUserLabs(ctx, dbgen.ListUserLabsParams{UserID: user, State: &state, PageLimit: 10}); len(rows) != 0 {
		t.Fatal("state filter ignored")
	}
}
