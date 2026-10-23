//go:build integration

package labs_test

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.yaml.in/yaml/v3"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/labs"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testdb"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

func nginxFixture(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("../../../../content/fixtures/nginx-one-replica.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := yaml.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	j, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(j)
}

type resource struct {
	Kind     string `json:"kind"`
	Metadata struct {
		Name            string            `json:"name"`
		Namespace       string            `json:"namespace"`
		UID             string            `json:"uid"`
		ResourceVersion string            `json:"resourceVersion"`
		Annotations     map[string]string `json:"annotations"`
	} `json:"metadata"`
}

func resources(t *testing.T, snap labs.Snapshot) []resource {
	t.Helper()
	out := make([]resource, 0, len(snap.Resources))
	for _, raw := range snap.Resources {
		var r resource
		if err := json.Unmarshal(raw, &r); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func count(rs []resource, kind string) int {
	n := 0
	for _, r := range rs {
		if r.Kind == kind {
			n++
		}
	}
	return n
}

type setup struct {
	svc    *labs.Service
	user   uuid.UUID
	lesson testdb.Lesson
}

func newSetup(t *testing.T) setup {
	t.Helper()
	pool := testdb.New(t)
	return setup{
		svc:    labs.NewService(pool),
		user:   testdb.CreateUser(t, pool, "learner"),
		lesson: testdb.CreateLaunchableLesson(t, pool, "scale", nginxFixture(t)),
	}
}

func TestCreateLoadsTheFixtureOnce(t *testing.T) {
	pool := testdb.New(t)
	svc := labs.NewService(pool)
	user := testdb.CreateUser(t, pool, "learner")
	lesson := testdb.CreateLaunchableLesson(t, pool, "scale", nginxFixture(t))

	lab, created, err := svc.Create(ctx, user, lesson.VersionID)
	if err != nil || !created {
		t.Fatalf("create: %v created=%v", err, created)
	}
	if lab.State != "active" || lab.Version != 1 || lab.SimulatorVersion != simulator.Version || lab.LessonID != "course-scale/scale" {
		t.Fatalf("lab %+v", lab)
	}
	if !lab.ExpiresAt.After(time.Now().Add(labs.IdleTTL - time.Minute)) {
		t.Fatalf("expiry %v", lab.ExpiresAt)
	}

	snap, err := svc.Snapshot(ctx, user, lab.ID)
	if err != nil {
		t.Fatal(err)
	}
	rs := resources(t, snap)
	if count(rs, "Node") != 3 || count(rs, "Deployment") != 1 || count(rs, "ReplicaSet") != 1 || count(rs, "Pod") != 1 || count(rs, "Namespace") != 4 {
		t.Fatalf("unexpected resources: %d total", len(rs))
	}
	for _, r := range rs {
		if r.Metadata.Annotations[simulator.SimulatedAnnotation] != "true" {
			t.Fatalf("%s/%s is not marked simulated", r.Kind, r.Metadata.Name)
		}
	}
	events, _, err := svc.Events(ctx, user, lab.ID, 0, 100)
	if err != nil || len(events) == 0 || events[0].Seq != 1 || events[0].Version != 1 {
		t.Fatalf("events %v %+v", err, events)
	}

	// Starting again returns the same active lab.
	again, created, err := svc.Create(ctx, user, lesson.VersionID)
	if err != nil || created || again.ID != lab.ID {
		t.Fatalf("second create: %v created=%v id=%v", err, created, again.ID)
	}
	var attempts int
	var activity int
	_ = pool.QueryRow(ctx, `SELECT attempts FROM lesson_progress WHERE user_id = $1`, user).Scan(&attempts)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM activity_events WHERE user_id = $1 AND type = 'lab-started'`, user).Scan(&activity)
	if attempts != 1 || activity != 1 {
		t.Fatalf("attempts %d, activity %d", attempts, activity)
	}
}

func TestConcurrentStartsCreateOneLab(t *testing.T) {
	s := newSetup(t)
	const n = 8
	ids := make(chan uuid.UUID, n)
	createdCount := make(chan bool, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Go(func() {
			lab, created, err := s.svc.Create(ctx, s.user, s.lesson.VersionID)
			if err != nil {
				t.Error(err)
				return
			}
			ids <- lab.ID
			createdCount <- created
		})
	}
	wg.Wait()
	close(ids)
	close(createdCount)
	distinct := map[uuid.UUID]bool{}
	for id := range ids {
		distinct[id] = true
	}
	created := 0
	for c := range createdCount {
		if c {
			created++
		}
	}
	if len(distinct) != 1 || created != 1 {
		t.Fatalf("%d distinct labs, %d created", len(distinct), created)
	}
}

func scale(replicas int32) func(*simulator.State) error {
	return func(st *simulator.State) error {
		d := st.Get(simulator.KindDeployment, "default", "nginx").(*simulator.Deployment)
		d.Spec.Replicas = &replicas
		return st.Put(d)
	}
}

func TestMutationsPersistAndSurviveARestart(t *testing.T) {
	pool := testdb.New(t)
	user := testdb.CreateUser(t, pool, "learner")
	lesson := testdb.CreateLaunchableLesson(t, pool, "scale", nginxFixture(t))
	first := labs.NewService(pool)
	lab, _, err := first.Create(ctx, user, lesson.VersionID)
	if err != nil {
		t.Fatal(err)
	}

	res, err := first.Simulate(ctx, user, lab.ID, true, scale(3))
	if err != nil || res.Version != 2 {
		t.Fatalf("scale: %v version %d", err, res.Version)
	}
	// A different service instance (an API restart) sees the same state.
	second := labs.NewService(pool)
	snap, err := second.Snapshot(ctx, user, lab.ID)
	if err != nil {
		t.Fatal(err)
	}
	if n := count(resources(t, snap), "Pod"); n != 3 || snap.Version != 2 {
		t.Fatalf("after restart: %d pods, version %d", n, snap.Version)
	}
	if !snap.SimTime.Equal(simulator.DefaultEpoch.Add(simulator.Tick)) {
		t.Fatalf("clock %v: a mutation is one tick", snap.SimTime)
	}

	// Reads don't move the clock or the version.
	res, err = second.Simulate(ctx, user, lab.ID, false, func(st *simulator.State) error {
		if len(st.List(simulator.KindPod, "default")) != 3 {
			return errors.New("read saw the wrong state")
		}
		return nil
	})
	if err != nil || res.Version != 2 {
		t.Fatalf("read: %v version %d", err, res.Version)
	}

	// Scaling down deletes rows; the restored state is stable under reconcile.
	if _, err := second.Simulate(ctx, user, lab.ID, true, scale(1)); err != nil {
		t.Fatal(err)
	}
	snap, _ = first.Snapshot(ctx, user, lab.ID)
	if n := count(resources(t, snap), "Pod"); n != 1 || snap.Version != 3 {
		t.Fatalf("after scale down: %d pods, version %d", n, snap.Version)
	}
	_, err = first.Simulate(ctx, user, lab.ID, false, func(st *simulator.State) error {
		rev := st.Revision
		if _, err := simulator.Reconcile(st); err != nil {
			return err
		}
		if st.Revision != rev {
			return errors.New("stored state was not a fixed point")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	events, _, _ := first.Events(ctx, user, lab.ID, 0, 100)
	sawScaleDown := false
	for _, e := range events {
		if e.Reason == "ScalingReplicaSet" && e.Version == 3 {
			sawScaleDown = true
		}
	}
	if !sawScaleDown {
		t.Fatal("scale-down event missing or not stamped with its version")
	}
}

func TestFailedMutationWritesNothing(t *testing.T) {
	s := newSetup(t)
	lab, _, _ := s.svc.Create(ctx, s.user, s.lesson.VersionID)
	before, _ := s.svc.Snapshot(ctx, s.user, lab.ID)
	_, err := s.svc.Simulate(ctx, s.user, lab.ID, true, func(st *simulator.State) error {
		if err := scale(5)(st); err != nil {
			return err
		}
		return errors.New("command failed half way")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	after, _ := s.svc.Snapshot(ctx, s.user, lab.ID)
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	if string(a) != string(b) {
		t.Fatal("a failed mutation changed stored state")
	}
}

func TestResetRestoresTheFixtureAndKeepsVersionsIncreasing(t *testing.T) {
	s := newSetup(t)
	lab, _, _ := s.svc.Create(ctx, s.user, s.lesson.VersionID)
	initial, _ := s.svc.Snapshot(ctx, s.user, lab.ID)
	if _, err := s.svc.Simulate(ctx, s.user, lab.ID, true, scale(4)); err != nil {
		t.Fatal(err)
	}
	mutated, _ := s.svc.Snapshot(ctx, s.user, lab.ID)

	reset, err := s.svc.Reset(ctx, s.user, lab.ID, "reset-key-0001")
	if err != nil || reset.Version != 3 {
		t.Fatalf("reset: %v version %d", err, reset.Version)
	}
	after, _ := s.svc.Snapshot(ctx, s.user, lab.ID)
	if !after.SimTime.Equal(initial.SimTime) {
		t.Fatalf("clock not reset: %v", after.SimTime)
	}
	// Same seed: same names and UIDs as the first load; resource versions only go up.
	want := map[string]string{}
	maxBefore := 0
	for _, r := range resources(t, initial) {
		want[r.Kind+"/"+r.Metadata.Namespace+"/"+r.Metadata.Name] = r.Metadata.UID
	}
	for _, r := range resources(t, mutated) {
		maxBefore = max(maxBefore, atoi(r.Metadata.ResourceVersion))
	}
	got := resources(t, after)
	if len(got) != len(want) {
		t.Fatalf("reset has %d resources, fixture had %d", len(got), len(want))
	}
	for _, r := range got {
		if want[r.Kind+"/"+r.Metadata.Namespace+"/"+r.Metadata.Name] != r.Metadata.UID {
			t.Fatalf("%s/%s differs after reset", r.Kind, r.Metadata.Name)
		}
		if atoi(r.Metadata.ResourceVersion) <= maxBefore {
			t.Fatalf("%s/%s resourceVersion %s reused (max before %d)", r.Kind, r.Metadata.Name, r.Metadata.ResourceVersion, maxBefore)
		}
	}
	events, _, _ := s.svc.Events(ctx, s.user, lab.ID, 0, 100)
	for _, e := range events {
		if e.Version != 3 {
			t.Fatalf("event from before the reset survived: %+v", e)
		}
	}

	// Retrying with the same key returns the stored result and doesn't reset again.
	retry, err := s.svc.Reset(ctx, s.user, lab.ID, "reset-key-0001")
	if err != nil || retry.Version != 3 {
		t.Fatalf("retry: %v version %d", err, retry.Version)
	}
	current, _ := s.svc.Get(ctx, s.user, lab.ID)
	if current.Version != 3 {
		t.Fatalf("retry reset again: version %d", current.Version)
	}
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

func TestIdempotencyKeyCannotBeReusedForAnotherLab(t *testing.T) {
	pool := testdb.New(t)
	svc := labs.NewService(pool)
	user := testdb.CreateUser(t, pool, "learner")
	a, _, _ := svc.Create(ctx, user, testdb.CreateLaunchableLesson(t, pool, "a", nginxFixture(t)).VersionID)
	b, _, _ := svc.Create(ctx, user, testdb.CreateLaunchableLesson(t, pool, "b", nginxFixture(t)).VersionID)
	if _, err := svc.Reset(ctx, user, a.ID, "shared-key-01"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Reset(ctx, user, b.ID, "shared-key-01"); !errors.Is(err, labs.ErrIdempotencyConflict) {
		t.Fatalf("got %v", err)
	}
	if lab, _ := svc.Get(ctx, user, b.ID); lab.Version != 1 {
		t.Fatal("conflicting request still reset the lab")
	}
}

func TestFinishEndsTheLab(t *testing.T) {
	s := newSetup(t)
	lab, _, _ := s.svc.Create(ctx, s.user, s.lesson.VersionID)
	done, err := s.svc.Finish(ctx, s.user, lab.ID, "finish-key-01")
	if err != nil || done.State != "abandoned" || done.EndedAt == nil {
		t.Fatalf("finish: %v %+v", err, done)
	}
	// A retry after the lab ended answers from the stored result.
	retry, err := s.svc.Finish(ctx, s.user, lab.ID, "finish-key-01")
	if err != nil || retry.State != "abandoned" {
		t.Fatalf("retry: %v %+v", err, retry)
	}
	// A new request on an ended lab is a conflict; so is any mutation.
	if _, err := s.svc.Finish(ctx, s.user, lab.ID, "finish-key-02"); !errors.Is(err, labs.ErrNotActive) {
		t.Fatalf("second finish: %v", err)
	}
	if _, err := s.svc.Simulate(ctx, s.user, lab.ID, true, scale(2)); !errors.Is(err, labs.ErrNotActive) {
		t.Fatalf("mutate ended lab: %v", err)
	}
	// Ended labs stay readable, and starting the lesson again gives a fresh lab.
	if _, err := s.svc.Snapshot(ctx, s.user, lab.ID); err != nil {
		t.Fatal(err)
	}
	fresh, created, err := s.svc.Create(ctx, s.user, s.lesson.VersionID)
	if err != nil || !created || fresh.ID == lab.ID {
		t.Fatalf("restart: %v created=%v", err, created)
	}
}

func TestOtherUsersCannotReachALab(t *testing.T) {
	pool := testdb.New(t)
	svc := labs.NewService(pool)
	owner := testdb.CreateUser(t, pool, "owner")
	intruder := testdb.CreateUser(t, pool, "intruder")
	lab, _, err := svc.Create(ctx, owner, testdb.CreateLaunchableLesson(t, pool, "scale", nginxFixture(t)).VersionID)
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]error{}
	_, checks["get"] = svc.Get(ctx, intruder, lab.ID)
	_, checks["snapshot"] = svc.Snapshot(ctx, intruder, lab.ID)
	_, _, checks["events"] = svc.Events(ctx, intruder, lab.ID, 0, 10)
	_, checks["resume"] = svc.Resume(ctx, intruder, lab.ID)
	_, checks["reset"] = svc.Reset(ctx, intruder, lab.ID, "intruder-key-1")
	_, checks["finish"] = svc.Finish(ctx, intruder, lab.ID, "intruder-key-2")
	_, checks["simulate"] = svc.Simulate(ctx, intruder, lab.ID, true, scale(9))
	for name, err := range checks {
		if !errors.Is(err, labs.ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if list, _, _ := svc.List(ctx, intruder, nil, nil, 10); len(list) != 0 {
		t.Error("intruder lists the owner's lab")
	}
	if l, _ := svc.Get(ctx, owner, lab.ID); l.Version != 1 || l.State != "active" {
		t.Fatalf("owner's lab changed: %+v", l)
	}
}

func TestExpiry(t *testing.T) {
	pool := testdb.New(t)
	svc := labs.NewService(pool)
	user := testdb.CreateUser(t, pool, "learner")
	idle, _, _ := svc.Create(ctx, user, testdb.CreateLaunchableLesson(t, pool, "idle", nginxFixture(t)).VersionID)
	swept, _, _ := svc.Create(ctx, user, testdb.CreateLaunchableLesson(t, pool, "swept", nginxFixture(t)).VersionID)
	upgraded, _, _ := svc.Create(ctx, user, testdb.CreateLaunchableLesson(t, pool, "upgraded", nginxFixture(t)).VersionID)
	if _, err := pool.Exec(ctx, `UPDATE lab_sessions SET expires_at = now() - interval '1 minute', started_at = now() - interval '15 days' WHERE id = ANY($1)`, []uuid.UUID{idle.ID, swept.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE lab_sessions SET simulator_version = 'kubernetes-1.36-subset-1' WHERE id = $1`, upgraded.ID); err != nil {
		t.Fatal(err)
	}

	// Reads report the lapsed lab as expired before the sweep.
	if l, _ := svc.Get(ctx, user, idle.ID); l.State != "expired" {
		t.Fatalf("get: %s", l.State)
	}
	// A write finds it, marks it expired, and says so.
	if _, err := svc.Resume(ctx, user, idle.ID); !errors.Is(err, labs.ErrExpired) {
		t.Fatalf("resume idle: %v", err)
	}
	if _, err := svc.Resume(ctx, user, upgraded.ID); !errors.Is(err, labs.ErrExpired) {
		t.Fatalf("resume upgraded: %v", err)
	}
	n, err := labs.ExpireIdle(ctx, pool)
	if err != nil || n != 1 {
		t.Fatalf("sweep expired %d: %v", n, err)
	}
	reasons := map[uuid.UUID]string{}
	rows, _ := pool.Query(ctx, `SELECT id, state || ':' || state_reason FROM lab_sessions WHERE user_id = $1`, user)
	for rows.Next() {
		var id uuid.UUID
		var r string
		_ = rows.Scan(&id, &r)
		reasons[id] = r
	}
	rows.Close()
	if reasons[idle.ID] != "expired:IDLE_TIMEOUT" || reasons[swept.ID] != "expired:IDLE_TIMEOUT" || reasons[upgraded.ID] != "expired:SIMULATOR_UPGRADED" {
		t.Fatalf("reasons %v", reasons)
	}
}

func TestOnlyTheCurrentVersionCanStart(t *testing.T) {
	pool := testdb.New(t)
	svc := labs.NewService(pool)
	user := testdb.CreateUser(t, pool, "learner")
	lesson := testdb.CreateLaunchableLesson(t, pool, "scale", nginxFixture(t))
	old := lesson.VersionID
	testdb.PublishVersion(t, pool, lesson.LessonID, 3, nginxFixture(t))
	if _, _, err := svc.Create(ctx, user, old); !errors.Is(err, labs.ErrStaleChallenge) {
		t.Fatalf("old version: %v", err)
	}
	if _, _, err := svc.Create(ctx, user, uuid.New()); !errors.Is(err, labs.ErrChallengeNotFound) {
		t.Fatalf("unknown version: %v", err)
	}
	draft := testdb.InsertVersion(t, pool, lesson.LessonID, 4, `{"fixture":{}}`)
	if _, _, err := svc.Create(ctx, user, draft); !errors.Is(err, labs.ErrChallengeNotFound) {
		t.Fatalf("draft version: %v", err)
	}
}

func TestListPaginatesWithCursor(t *testing.T) {
	pool := testdb.New(t)
	svc := labs.NewService(pool)
	user := testdb.CreateUser(t, pool, "learner")
	for _, slug := range []string{"one", "two", "three"} {
		if _, _, err := svc.Create(ctx, user, testdb.CreateLaunchableLesson(t, pool, slug, nginxFixture(t)).VersionID); err != nil {
			t.Fatal(err)
		}
	}
	page, next, err := svc.List(ctx, user, nil, nil, 2)
	if err != nil || len(page) != 2 || next == nil {
		t.Fatalf("page 1: %v %d %v", err, len(page), next)
	}
	rest, next, err := svc.List(ctx, user, nil, next, 2)
	if err != nil || len(rest) != 1 || next != nil || rest[0].ID == page[0].ID || rest[0].ID == page[1].ID {
		t.Fatalf("page 2: %v %d %v", err, len(rest), next)
	}
	active := "abandoned"
	if none, _, _ := svc.List(ctx, user, &active, nil, 10); len(none) != 0 {
		t.Fatal("state filter ignored")
	}
}
