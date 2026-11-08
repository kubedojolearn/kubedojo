//go:build integration

package labs_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/labs"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testdb"
)

const gradedChallenge = `{
  "version": 2,
  "goals": [
    {"id": "scaled", "type": "state", "description": "nginx runs 3 ready replicas", "failureMessage": "Check kubectl get deploy nginx.",
     "target": {"kind": "Deployment", "namespace": "default", "name": "nginx"},
     "assertions": [{"path": "spec.replicas", "operator": "equals", "value": 3}, {"path": "status.readyReplicas", "operator": "equals", "value": 3}]},
    {"id": "node-count", "type": "answer", "description": "Count the nodes", "prompt": "How many nodes are there?", "input": "number",
     "expected": {"count": {"kind": "Node"}}},
    {"id": "described", "type": "observation", "description": "Describe the Deployment",
     "command": {"verb": "describe", "kind": "Deployment", "name": "nginx"}},
    {"id": "reason", "type": "answer", "description": "Pick the controller", "prompt": "Which controller creates the Pods?", "input": "choice",
     "choices": ["Deployment", "ReplicaSet", "Scheduler"], "expected": {"static": "ReplicaSet"}}
  ],
  "hints": ["Try kubectl get deployments.", "kubectl scale changes replicas.", "kubectl scale deployment nginx --replicas=3"],
  "reward": {"baseXP": 100, "firstTryBonusXP": 25, "hintPenaltyPerHintXP": 10}
}`

type gradingSetup struct {
	svc    *labs.Service
	pool   *pgxpool.Pool
	lesson testdb.Lesson
}

func newGrading(t *testing.T) gradingSetup {
	t.Helper()
	pool := testdb.New(t)
	if _, err := pool.Exec(ctx, `INSERT INTO achievement_definitions (key, title, description, icon, condition, display_order) VALUES
		('first-lesson', 'First steps', 'Complete your first lesson.', 'Footprints', '{"type":"lessons_completed","count":1}', 1),
		('first-try', 'First try', 'Pass a lesson on your first check.', 'Target', '{"type":"first_try_passes","count":1}', 2),
		('no-hints', 'No help needed', 'Complete a lesson without a hint.', 'Lightbulb', '{"type":"lessons_without_hints","count":1}', 3)`); err != nil {
		t.Fatal(err)
	}
	return gradingSetup{svc: labs.NewService(pool), pool: pool, lesson: testdb.CreateLaunchableChallenge(t, pool, "graded", gradedChallenge, nginxFixture(t))}
}

func (g gradingSetup) start(t *testing.T, user uuid.UUID) labs.Lab {
	t.Helper()
	lab, _, err := g.svc.Create(ctx, user, g.lesson.VersionID)
	if err != nil {
		t.Fatal(err)
	}
	return lab
}

func (g gradingSetup) run(t *testing.T, user uuid.UUID, lab labs.Lab, cmds ...string) {
	t.Helper()
	for _, c := range cmds {
		if r, err := g.svc.RunCommand(ctx, user, lab.ID, uuid.New(), c); err != nil || r.ExitCode != 0 {
			t.Fatalf("%s: %v %+v", c, err, r)
		}
	}
}

func TestGradingCompletesTheLabOnce(t *testing.T) {
	g := newGrading(t)
	user := testdb.CreateUser(t, g.pool, "learner")
	lab := g.start(t, user)

	// Nothing done yet: the check fails, the lab stays active, and nothing is revealed.
	first, err := g.svc.Validate(ctx, user, lab.ID, "check-0001")
	if err != nil || first.Passed || first.Score != 0 || first.LabState != "active" || first.XPAwarded != 0 {
		t.Fatalf("first check: %v %+v", err, first)
	}
	if first.Goals[0].Message != "Check kubectl get deploy nginx." {
		t.Fatalf("failure message %q", first.Goals[0].Message)
	}

	// Hints come in order; a hint seen again costs nothing more.
	if _, err := g.svc.RevealHint(ctx, user, lab.ID, 1); !errors.Is(err, labs.ErrHintOrder) {
		t.Fatalf("out of order: %v", err)
	}
	if h, err := g.svc.RevealHint(ctx, user, lab.ID, 0); err != nil || h.HintsUsed != 1 || h.Text != "Try kubectl get deployments." || h.PenaltyXP != 10 {
		t.Fatalf("hint 0: %v %+v", err, h)
	}
	if h, _ := g.svc.RevealHint(ctx, user, lab.ID, 0); h.HintsUsed != 1 {
		t.Fatalf("revealing again charged another hint: %+v", h)
	}
	if _, err := g.svc.RevealHint(ctx, user, lab.ID, 3); !errors.Is(err, labs.ErrHintNotFound) {
		t.Fatalf("missing hint: %v", err)
	}

	// Answers are checked for shape, not correctness, when submitted.
	if _, err := g.svc.SubmitAnswer(ctx, user, lab.ID, "node-count", "three"); !errors.Is(err, labs.ErrInvalidAnswer) {
		t.Fatalf("non-number: %v", err)
	}
	if _, err := g.svc.SubmitAnswer(ctx, user, lab.ID, "scaled", "3"); !errors.Is(err, labs.ErrGoalNotFound) {
		t.Fatalf("state goal: %v", err)
	}
	if st, err := g.svc.SubmitAnswer(ctx, user, lab.ID, "node-count", "2"); err != nil || st.SubmissionsRemaining != 9 {
		t.Fatalf("wrong answer: %v %+v", err, st)
	}

	// Do the work: scale, look, answer.
	g.run(t, user, lab, "kubectl scale deployment nginx --replicas=3", "kubectl describe deploy nginx")
	if _, err := g.svc.SubmitAnswer(ctx, user, lab.ID, "node-count", " 3 "); err != nil {
		t.Fatal(err)
	}
	if _, err := g.svc.SubmitAnswer(ctx, user, lab.ID, "reason", "ReplicaSet"); err != nil {
		t.Fatal(err)
	}

	passed, err := g.svc.Validate(ctx, user, lab.ID, "check-0002")
	if err != nil || !passed.Passed || passed.Score != 100 || passed.LabState != "completed" {
		t.Fatalf("passing check: %v %+v", err, passed)
	}
	// Not the first check of the lesson, one hint: 100 - 10.
	if passed.XPAwarded != 90 {
		t.Fatalf("xp %d, want 90", passed.XPAwarded)
	}
	if len(passed.Achievements) != 1 || passed.Achievements[0].Key != "first-lesson" {
		t.Fatalf("achievements %+v", passed.Achievements)
	}
	var totalXP, completed, streak int
	_ = g.pool.QueryRow(ctx, `SELECT total_xp, completed_lessons, current_streak FROM user_totals WHERE user_id = $1`, user).Scan(&totalXP, &completed, &streak)
	if totalXP != 90 || completed != 1 || streak != 1 {
		t.Fatalf("totals xp=%d completed=%d streak=%d", totalXP, completed, streak)
	}

	// A retry of the passing check returns the stored result; the lab is read-only now.
	again, err := g.svc.Validate(ctx, user, lab.ID, "check-0002")
	if err != nil || again.AttemptID != passed.AttemptID || again.XPAwarded != 90 {
		t.Fatalf("replay: %v %+v", err, again)
	}
	if _, err := g.svc.Validate(ctx, user, lab.ID, "check-0003"); !errors.Is(err, labs.ErrNotActive) {
		t.Fatalf("check on a completed lab: %v", err)
	}
	if _, err := g.svc.RunCommand(ctx, user, lab.ID, uuid.New(), "kubectl get pods"); !errors.Is(err, labs.ErrNotActive) {
		t.Fatalf("command on a completed lab: %v", err)
	}

	// The goals panel restores after a reload, without expected answers.
	goals, err := g.svc.Goals(ctx, user, lab.ID)
	if err != nil || goals.LastAttempt == nil || !goals.LastAttempt.Passed || goals.LastAttempt.XPAwarded != 90 || len(goals.Hints) != 1 || goals.HintCount != 3 {
		t.Fatalf("goals: %v %+v", err, goals)
	}
	for _, a := range goals.Answers {
		if a.GoalID == "node-count" && (a.Submissions != 2 || *a.LastAnswer != "3") {
			t.Fatalf("answer status %+v", a)
		}
	}

	// Passing the lesson again in a new lab completes the lab but awards nothing twice.
	lab2 := g.start(t, user)
	g.run(t, user, lab2, "kubectl scale deployment nginx --replicas=3", "kubectl describe deploy nginx")
	_, _ = g.svc.SubmitAnswer(ctx, user, lab2.ID, "node-count", "3")
	_, _ = g.svc.SubmitAnswer(ctx, user, lab2.ID, "reason", "ReplicaSet")
	repeat, err := g.svc.Validate(ctx, user, lab2.ID, "check-0004")
	if err != nil || !repeat.Passed || repeat.XPAwarded != 0 || len(repeat.Achievements) != 0 {
		t.Fatalf("repeat pass: %v %+v", err, repeat)
	}
	_ = g.pool.QueryRow(ctx, `SELECT total_xp, completed_lessons FROM user_totals WHERE user_id = $1`, user).Scan(&totalXP, &completed)
	if totalXP != 90 || completed != 1 {
		t.Fatalf("after repeat xp=%d completed=%d", totalXP, completed)
	}
}

func TestFirstTryPassEarnsTheBonus(t *testing.T) {
	g := newGrading(t)
	user := testdb.CreateUser(t, g.pool, "ace")
	lab := g.start(t, user)
	g.run(t, user, lab, "kubectl scale deployment nginx --replicas=3", "kubectl describe deployment/nginx")
	_, _ = g.svc.SubmitAnswer(ctx, user, lab.ID, "node-count", "3")
	_, _ = g.svc.SubmitAnswer(ctx, user, lab.ID, "reason", "ReplicaSet")
	res, err := g.svc.Validate(ctx, user, lab.ID, "first-check")
	if err != nil || !res.Passed || res.XPAwarded != 125 {
		t.Fatalf("%v %+v", err, res)
	}
	keys := map[string]bool{}
	for _, a := range res.Achievements {
		keys[a.Key] = true
	}
	if !keys["first-lesson"] || !keys["first-try"] || !keys["no-hints"] {
		t.Fatalf("achievements %+v", res.Achievements)
	}
}

func TestAnswerLimitsAndOwnership(t *testing.T) {
	g := newGrading(t)
	user := testdb.CreateUser(t, g.pool, "learner")
	other := testdb.CreateUser(t, g.pool, "other")
	lab := g.start(t, user)
	// Three choices allow two submissions, so guessing every choice is impossible.
	for i, choice := range []string{"Deployment", "Scheduler"} {
		st, err := g.svc.SubmitAnswer(ctx, user, lab.ID, "reason", choice)
		if err != nil || st.SubmissionsRemaining != 1-i {
			t.Fatalf("submission %d: %v %+v", i, err, st)
		}
	}
	if _, err := g.svc.SubmitAnswer(ctx, user, lab.ID, "reason", "ReplicaSet"); !errors.Is(err, labs.ErrNoSubmissionsLeft) {
		t.Fatalf("third choice: %v", err)
	}
	if _, err := g.svc.SubmitAnswer(ctx, user, lab.ID, "reason", "Kubelet"); !errors.Is(err, labs.ErrInvalidAnswer) && !errors.Is(err, labs.ErrNoSubmissionsLeft) {
		t.Fatalf("not a choice: %v", err)
	}
	if _, err := g.svc.Validate(ctx, other, lab.ID, "intruder-check"); !errors.Is(err, labs.ErrNotFound) {
		t.Fatalf("other user check: %v", err)
	}
	if _, err := g.svc.Goals(ctx, other, lab.ID); !errors.Is(err, labs.ErrNotFound) {
		t.Fatalf("other user goals: %v", err)
	}
	if _, err := g.svc.RevealHint(ctx, other, lab.ID, 0); !errors.Is(err, labs.ErrNotFound) {
		t.Fatalf("other user hint: %v", err)
	}
	// Keys are per operation: a check's key doesn't block a reset with the same key.
	if _, err := g.svc.Validate(ctx, user, lab.ID, "shared-key-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.svc.Reset(ctx, user, lab.ID, "shared-key-1"); err != nil {
		t.Fatalf("a validate key doesn't block a reset key: %v", err)
	}
}
