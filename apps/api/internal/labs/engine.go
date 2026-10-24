package labs

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"sync"

	"github.com/google/uuid"

	dbgen "github.com/kubedojolearn/kubedojo/apps/api/db/generated"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// The simulator is pure (ADR-0006); this file moves its state in and out of PostgreSQL.

// fixtureCache holds parsed fixtures by challenge version. Published versions are immutable, so
// entries never go stale; the cache is cleared when full rather than tracking recency.
type fixtureCache struct {
	mu sync.Mutex
	m  map[uuid.UUID]simulator.Fixture
}

const fixtureCacheSize = 256

func (c *fixtureCache) get(ctx context.Context, q *dbgen.Queries, versionID uuid.UUID) (simulator.Fixture, error) {
	c.mu.Lock()
	f, ok := c.m[versionID]
	c.mu.Unlock()
	if ok {
		return f, nil
	}
	spec, err := q.GetChallengeVersionSpec(ctx, versionID)
	if err != nil {
		return f, fmt.Errorf("load challenge version: %w", err)
	}
	if f, err = parseFixture(spec); err != nil {
		return f, err
	}
	c.mu.Lock()
	if c.m == nil || len(c.m) >= fixtureCacheSize {
		c.m = map[uuid.UUID]simulator.Fixture{}
	}
	c.m[versionID] = f
	c.mu.Unlock()
	return f, nil
}

// parseFixture reads the fixture from a stored challenge spec: {"challenge": …, "fixture": …}.
func parseFixture(spec json.RawMessage) (simulator.Fixture, error) {
	var doc struct {
		Fixture *simulator.Fixture `json:"fixture"`
	}
	if err := json.Unmarshal(spec, &doc); err != nil {
		return simulator.Fixture{}, fmt.Errorf("challenge spec: %w", err)
	}
	if doc.Fixture == nil {
		return simulator.Fixture{}, fmt.Errorf("challenge spec has no fixture")
	}
	return *doc.Fixture, nil
}

// newSeed returns a random non-negative seed (lab_sessions.seed is a signed bigint).
func newSeed() (int64, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return 0, err
	}
	return n.Int64(), nil
}

// The database stores these unsigned simulator counters as non-negative bigints.
func toUint(v int64) uint64 { return uint64(max(v, 0)) } //nolint:gosec // non-negative by construction

func toInt(v uint64) int64 {
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(v)
}

// LoadState rebuilds a lab's simulator state from its stored resources.
func LoadState(ctx context.Context, q *dbgen.Queries, s dbgen.LabSession, f simulator.Fixture) (*simulator.State, error) {
	state := simulator.NewState(s.SimTime, toUint(s.Seed), simulator.NewImageCatalog(f.ImageCatalog))
	state.Gen.Counter = toUint(s.GeneratorCounter)
	state.CurrentNamespace = s.CurrentNamespace
	rows, err := q.ListLabResources(ctx, s.ID)
	if err != nil {
		return nil, fmt.Errorf("load lab resources: %w", err)
	}
	objs := make([]simulator.Object, 0, len(rows))
	for _, r := range rows {
		o, err := simulator.DecodeStored(simulator.Stored{
			Kind: r.Kind, Namespace: r.Namespace, Name: r.Name, APIVersion: r.ApiVersion, UID: r.Uid.String(),
			ResourceVersion: r.ResourceVersion, Generation: r.Generation,
			Metadata: r.Metadata, Spec: r.Spec, Status: r.Status, Extra: r.Extra,
		})
		if err != nil {
			return nil, fmt.Errorf("decode %s %s/%s: %w", r.Kind, r.Namespace, r.Name, err)
		}
		objs = append(objs, o)
	}
	if err := state.Restore(objs); err != nil {
		return nil, err
	}
	state.Revision = max(state.Revision, s.ResourceRevision)
	return state, nil
}

// persisted reports what persist wrote.
type persisted struct {
	Events int
}

// persist writes the state's changed and deleted objects, its new events (stamped with
// version) and the engine counters. It must run inside the lab's transaction.
func persist(ctx context.Context, q *dbgen.Queries, s dbgen.LabSession, state *simulator.State, version int64) (persisted, error) {
	upserted, deleted := state.Changes()
	for _, k := range deleted {
		if err := q.DeleteLabResource(ctx, dbgen.DeleteLabResourceParams{SessionID: s.ID, Kind: k.Kind, Namespace: k.Namespace, Name: k.Name}); err != nil {
			return persisted{}, fmt.Errorf("delete %s: %w", k, err)
		}
	}
	for _, o := range upserted {
		st, err := simulator.Encode(o)
		if err != nil {
			return persisted{}, err
		}
		uid, err := uuid.Parse(st.UID)
		if err != nil {
			return persisted{}, fmt.Errorf("%s has a malformed uid: %w", simulator.KeyOf(o), err)
		}
		if err := q.UpsertLabResource(ctx, dbgen.UpsertLabResourceParams{
			SessionID: s.ID, Kind: st.Kind, Namespace: st.Namespace, Name: st.Name, ApiVersion: st.APIVersion, Uid: uid,
			ResourceVersion: st.ResourceVersion, Generation: st.Generation,
			Metadata: st.Metadata, Spec: st.Spec, Status: st.Status, Extra: st.Extra,
		}); err != nil {
			return persisted{}, fmt.Errorf("save %s: %w", simulator.KeyOf(o), err)
		}
	}

	seq := s.EventSeq
	events := state.Events()
	for _, e := range events {
		seq++
		if err := q.InsertLabEvent(ctx, dbgen.InsertLabEventParams{
			SessionID: s.ID, Seq: seq, Version: version, Type: e.Type, Reason: truncate(e.Reason, 64),
			InvolvedKind: e.Kind, InvolvedNamespace: e.Namespace, InvolvedName: e.Name,
			Message: truncate(e.Message, 1024), OccurredAt: e.Time,
		}); err != nil {
			return persisted{}, fmt.Errorf("save event: %w", err)
		}
	}
	if keep := int64(state.Limits.MaxEvents); len(events) > 0 && seq > keep {
		if err := q.PruneLabEvents(ctx, dbgen.PruneLabEventsParams{SessionID: s.ID, ThroughSeq: seq - keep}); err != nil {
			return persisted{}, fmt.Errorf("prune events: %w", err)
		}
	}

	if err := q.SaveLabEngineState(ctx, dbgen.SaveLabEngineStateParams{
		ID: s.ID, SimTime: state.Now, GeneratorCounter: toInt(state.Gen.Counter), ResourceRevision: state.Revision,
		EventSeq: seq, CurrentNamespace: state.CurrentNamespace,
	}); err != nil {
		return persisted{}, fmt.Errorf("save engine state: %w", err)
	}
	return persisted{Events: len(events)}, nil
}

// truncate shortens s to at most n bytes without splitting a UTF-8 sequence.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && s[n]&0xC0 == 0x80 {
		n--
	}
	return s[:n]
}
