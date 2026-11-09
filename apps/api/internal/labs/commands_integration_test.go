//go:build integration

package labs_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/labs"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testdb"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// fixtureWithSecret adds a training Secret to the nginx fixture.
func fixtureWithSecret(t *testing.T) string {
	t.Helper()
	var f map[string]any
	if err := json.Unmarshal([]byte(nginxFixture(t)), &f); err != nil {
		t.Fatal(err)
	}
	f["resources"] = append(f["resources"].([]any), map[string]any{
		"apiVersion": "v1", "kind": "Secret",
		"metadata":   map[string]any{"name": "db-creds", "namespace": "default"},
		"stringData": map[string]any{"password": "training-only"},
	})
	b, _ := json.Marshal(f)
	return string(b)
}

func TestCommandsRunReadOnlyAndAreRecorded(t *testing.T) {
	s := newSetup(t)
	lab, _, _ := s.svc.Create(ctx, s.user, s.lesson.VersionID)
	before, _ := s.svc.Snapshot(ctx, s.user, lab.ID)

	res, err := s.svc.RunCommand(ctx, s.user, lab.ID, uuid.New(), "kubectl get pods")
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 || !strings.Contains(res.Stdout, "Running") || res.Sequence != 1 || res.Mutated || res.SnapshotVersion != 1 {
		t.Fatalf("result %+v", res)
	}
	// Events stored at lab start are visible to commands.
	ev, err := s.svc.RunCommand(ctx, s.user, lab.ID, uuid.New(), "kubectl get events")
	if err != nil || !strings.Contains(ev.Stdout, "Scheduled") || ev.Sequence != 2 {
		t.Fatalf("events: %v %+v", err, ev)
	}
	bad, err := s.svc.RunCommand(ctx, s.user, lab.ID, uuid.New(), "kubectl get pods; rm -rf /")
	if err != nil || bad.ExitCode == 0 || bad.ErrorCode == nil || *bad.ErrorCode != "UNSUPPORTED_COMMAND" {
		t.Fatalf("injection: %v %+v", err, bad)
	}

	after, _ := s.svc.Snapshot(ctx, s.user, lab.ID)
	a, _ := json.Marshal(before)
	b, _ := json.Marshal(after)
	if string(a) != string(b) {
		t.Fatal("read commands changed the lab (state, version or clock)")
	}
	l, _ := s.svc.Get(ctx, s.user, lab.ID)
	if l.CommandSeq != 3 || l.Version != 1 {
		t.Fatalf("commandSeq %d version %d", l.CommandSeq, l.Version)
	}

	history, next, err := s.svc.Commands(ctx, s.user, lab.ID, nil, 2)
	if err != nil || len(history) != 2 || history[0].Sequence != 3 || next == nil {
		t.Fatalf("history: %v %d %v", err, len(history), next)
	}
	rest, next, _ := s.svc.Commands(ctx, s.user, lab.ID, next, 2)
	if len(rest) != 1 || rest[0].Sequence != 1 || next != nil {
		t.Fatalf("history page 2: %d %v", len(rest), next)
	}
}

func TestCommandsAreDeduplicatedByRequestID(t *testing.T) {
	s := newSetup(t)
	lab, _, _ := s.svc.Create(ctx, s.user, s.lesson.VersionID)
	id := uuid.New()
	first, err := s.svc.RunCommand(ctx, s.user, lab.ID, id, "kubectl get pods")
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.svc.RunCommand(ctx, s.user, lab.ID, id, "kubectl get nodes")
	if err != nil || again.Sequence != first.Sequence || again.Input != "kubectl get pods" {
		t.Fatalf("replay: %v %+v", err, again)
	}

	// Concurrent retries of one request run it once.
	concurrent := uuid.New()
	var wg sync.WaitGroup
	seqs := make(chan int64, 8)
	for i := 0; i < 8; i++ {
		wg.Go(func() {
			r, err := s.svc.RunCommand(ctx, s.user, lab.ID, concurrent, "kubectl get deploy")
			if err != nil {
				t.Error(err)
				return
			}
			seqs <- r.Sequence
		})
	}
	wg.Wait()
	close(seqs)
	distinct := map[int64]bool{}
	for q := range seqs {
		distinct[q] = true
	}
	if len(distinct) != 1 {
		t.Fatalf("one request ran %d times", len(distinct))
	}
	if l, _ := s.svc.Get(ctx, s.user, lab.ID); l.CommandSeq != 2 {
		t.Fatalf("commandSeq %d", l.CommandSeq)
	}
}

func TestSecretValuesAreNotKeptInHistory(t *testing.T) {
	pool := testdb.New(t)
	svc := labs.NewService(pool)
	user := testdb.CreateUser(t, pool, "learner")
	lab, _, err := svc.Create(ctx, user, testdb.CreateLaunchableLesson(t, pool, "secrets", fixtureWithSecret(t)).VersionID)
	if err != nil {
		t.Fatal(err)
	}
	res, err := svc.RunCommand(ctx, user, lab.ID, uuid.New(), "kubectl get secret db-creds -o yaml")
	encoded := "dHJhaW5pbmctb25seQ==" // base64("training-only")
	if err != nil || !strings.Contains(res.Stdout, encoded) {
		t.Fatalf("learner should see their Secret: %v\n%s", err, res.Stdout)
	}
	history, _, _ := svc.Commands(ctx, user, lab.ID, nil, 10)
	if strings.Contains(history[0].Stdout, encoded) || !strings.Contains(history[0].Stdout, "<redacted>") {
		t.Fatalf("history kept the value:\n%s", history[0].Stdout)
	}
	var stored int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM command_executions WHERE stdout LIKE '%' || $1 || '%'`, encoded).Scan(&stored)
	if stored != 0 {
		t.Fatal("a Secret value was stored")
	}
}

func TestCommandsRespectOwnershipAndLifecycle(t *testing.T) {
	pool := testdb.New(t)
	svc := labs.NewService(pool)
	owner := testdb.CreateUser(t, pool, "owner")
	intruder := testdb.CreateUser(t, pool, "intruder")
	lab, _, _ := svc.Create(ctx, owner, testdb.CreateLaunchableLesson(t, pool, "x", nginxFixture(t)).VersionID)
	id := uuid.New()
	if _, err := svc.RunCommand(ctx, owner, lab.ID, id, "kubectl get pods"); err != nil {
		t.Fatal(err)
	}
	// Another user can neither run commands nor replay the owner's request ID.
	if _, err := svc.RunCommand(ctx, intruder, lab.ID, id, "kubectl get pods"); !errors.Is(err, labs.ErrNotFound) {
		t.Fatalf("intruder replay: %v", err)
	}
	if _, _, err := svc.Commands(ctx, intruder, lab.ID, nil, 10); !errors.Is(err, labs.ErrNotFound) {
		t.Fatalf("intruder history: %v", err)
	}
	if _, err := svc.RunCommand(ctx, owner, lab.ID, uuid.New(), "kubectl\x00"); !errors.Is(err, labs.ErrInvalidInput) {
		t.Fatalf("NUL input: %v", err)
	}

	if _, err := svc.Finish(ctx, owner, lab.ID, "finish-key-01"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RunCommand(ctx, owner, lab.ID, uuid.New(), "kubectl get pods"); !errors.Is(err, labs.ErrNotActive) {
		t.Fatalf("ended lab: %v", err)
	}
	// A retry of a command that already ran still gets its result.
	if r, err := svc.RunCommand(ctx, owner, lab.ID, id, "kubectl get pods"); err != nil || r.Sequence != 1 {
		t.Fatalf("replay after end: %v %+v", err, r)
	}
}

func TestMutatingCommandsCommitState(t *testing.T) {
	s := newSetup(t)
	lab, _, _ := s.svc.Create(ctx, s.user, s.lesson.VersionID)
	before, _ := s.svc.Snapshot(ctx, s.user, lab.ID)

	res, err := s.svc.RunCommand(ctx, s.user, lab.ID, uuid.New(), "kubectl scale deployment nginx --replicas=3")
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 0 || !res.Mutated || res.SnapshotVersion != 2 || res.Stdout != "deployment.apps/nginx scaled\n" {
		t.Fatalf("scale: %+v", res)
	}
	after, _ := s.svc.Snapshot(ctx, s.user, lab.ID)
	if after.Version != 2 || !after.SimTime.Equal(before.SimTime.Add(10e9)) {
		t.Fatalf("version %d simTime %s (was %s)", after.Version, after.SimTime, before.SimTime)
	}
	pods, _ := s.svc.RunCommand(ctx, s.user, lab.ID, uuid.New(), "kubectl get pods --no-headers")
	if n := strings.Count(pods.Stdout, "Running"); n != 3 {
		t.Fatalf("want 3 running Pods after a reload from the database, got %d:\n%s", n, pods.Stdout)
	}
	// The controllers' events were stored with the new version.
	events, _, _ := s.svc.Events(ctx, s.user, lab.ID, 0, 500)
	scaled := false
	for _, e := range events {
		if e.Reason == "ScalingReplicaSet" && e.Version == 2 && strings.Contains(e.Message, "from 1 to 3") {
			scaled = true
		}
	}
	if !scaled {
		t.Fatal("no ScalingReplicaSet event at version 2")
	}
	var outbox int
	_ = s.pool.QueryRow(ctx, `SELECT count(*) FROM outbox_events WHERE aggregate_id = $1 AND event_type = 'lab.state_changed'`, lab.ID).Scan(&outbox)
	if outbox != 1 {
		t.Fatalf("outbox events: %d", outbox)
	}

	// A refused mutation changes nothing: no version, no clock, no objects.
	bad, err := s.svc.RunCommand(ctx, s.user, lab.ID, uuid.New(), "kubectl delete namespace default")
	if err != nil || bad.ExitCode == 0 || bad.Mutated || bad.SnapshotVersion != 2 {
		t.Fatalf("refused delete: %v %+v", err, bad)
	}
	if again, _ := s.svc.Snapshot(ctx, s.user, lab.ID); again.Version != 2 || !again.SimTime.Equal(after.SimTime) {
		t.Fatalf("refused command changed the lab: version %d", again.Version)
	}

	// The kubeconfig namespace persists without moving the clock.
	if r, err := s.svc.RunCommand(ctx, s.user, lab.ID, uuid.New(), "kubectl config set-context --current --namespace=kube-system"); err != nil || !r.Mutated {
		t.Fatalf("set-context: %v %+v", err, r)
	}
	snap, _ := s.svc.Snapshot(ctx, s.user, lab.ID)
	if snap.CurrentNamespace != "kube-system" || !snap.SimTime.Equal(after.SimTime) {
		t.Fatalf("namespace %q simTime %s", snap.CurrentNamespace, snap.SimTime)
	}

	// Reset brings back the fixture, keeping the version moving forward.
	reset, err := s.svc.Reset(ctx, s.user, lab.ID, "reset-key-0001")
	if err != nil {
		t.Fatal(err)
	}
	if reset.Version <= snap.Version {
		t.Fatalf("reset version %d after %d", reset.Version, snap.Version)
	}
	pods, _ = s.svc.RunCommand(ctx, s.user, lab.ID, uuid.New(), "kubectl get pods -n default --no-headers")
	if n := strings.Count(pods.Stdout, "Running"); n != 1 {
		t.Fatalf("after reset want 1 Pod, got %d", n)
	}
}

func TestConcurrentCommandsSerialize(t *testing.T) {
	s := newSetup(t)
	lab, _, _ := s.svc.Create(ctx, s.user, s.lesson.VersionID)
	const n = 6
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Go(func() {
			r, err := s.svc.RunCommand(ctx, s.user, lab.ID, uuid.New(), fmt.Sprintf("kubectl create configmap cm-%d --from-literal=k=v", i))
			if err != nil || r.ExitCode != 0 || !r.Mutated {
				t.Errorf("create %d: %v %+v", i, err, r)
			}
		})
	}
	wg.Wait()
	l, _ := s.svc.Get(ctx, s.user, lab.ID)
	if l.Version != 1+n || l.CommandSeq != n {
		t.Fatalf("version %d commandSeq %d", l.Version, l.CommandSeq)
	}
	cms, _ := s.svc.RunCommand(ctx, s.user, lab.ID, uuid.New(), "kubectl get configmaps --no-headers")
	if got := strings.Count(cms.Stdout, "cm-"); got != n {
		t.Fatalf("want %d ConfigMaps, got %d:\n%s", n, got, cms.Stdout)
	}
	// Every mutation moved the clock exactly one tick.
	snap, _ := s.svc.Snapshot(ctx, s.user, lab.ID)
	if want := simulator.DefaultEpoch.Add(time.Duration(n) * simulator.Tick); !snap.SimTime.Equal(want) {
		t.Fatalf("simTime %s, want %s", snap.SimTime, want)
	}
}
