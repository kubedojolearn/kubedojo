package grading

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/content"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/kubectl"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// spec is the part of a stored challenge this test needs.
type spec struct {
	Challenge struct {
		Goals        []Goal `json:"goals"`
		Hints        []string
		InitialFiles []struct{ Path, Content string } `json:"initialFiles"`
		Solution     struct {
			Files    []struct{ Path, Content string } `json:"files"`
			Commands []string                         `json:"commands"`
			Answers  map[string]string                `json:"answers"`
		} `json:"solution"`
	} `json:"challenge"`
	Fixture simulator.Fixture `json:"fixture"`
}

// newLab loads a challenge's fixture the way a lab starts: events from loading are already
// stored, and the state holds none of its own.
func newLab(t *testing.T, s spec) kubectl.Env {
	t.Helper()
	st, err := simulator.Load(s.Fixture, 42)
	if err != nil {
		t.Fatal(err)
	}
	events := append([]simulator.Event{}, st.Events()...)
	restored := simulator.NewState(st.Now, st.Gen.Seed, st.Images)
	if err := restored.Restore(st.All()); err != nil {
		t.Fatal(err)
	}
	restored.Gen, restored.Revision, restored.CurrentNamespace = st.Gen, st.Revision, st.CurrentNamespace
	files := map[string]string{}
	for _, f := range s.Challenge.InitialFiles {
		files[f.Path] = f.Content
	}
	return kubectl.Env{State: restored, Events: events, Files: files}
}

// TestReferenceSolutions replays every lesson's hidden solution (ADR-0006: output is a function of
// fixture, seed and commands) and checks that the lesson can't be passed by doing nothing, that
// every solution command is supported, and that the solution passes every goal.
func TestReferenceSolutions(t *testing.T) {
	bundle, err := content.Load("../../../../content")
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(bundle.Challenges))
	for id := range bundle.Challenges {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			var s spec
			if err := json.Unmarshal(bundle.Challenges[id].Spec, &s); err != nil {
				t.Fatal(err)
			}
			env := newLab(t, s)

			before := Evaluate(env.State, s.Challenge.Goals, Inputs{Answers: s.Challenge.Solution.Answers})
			if Score(before) == 100 && !onlyAnswerGoals(s.Challenge.Goals) {
				t.Errorf("every goal passes before the learner does anything")
			}

			for _, f := range s.Challenge.Solution.Files {
				env.Files[f.Path] = f.Content
			}
			var commands []kubectl.Parsed
			for _, cmd := range s.Challenge.Solution.Commands {
				r := kubectl.Execute(cmd, env)
				if r.Err != nil {
					t.Fatalf("%s: simulator error: %v", cmd, r.Err)
				}
				if r.ErrorCode != "" {
					t.Errorf("%s: %s (exit %d)\n%s", cmd, r.ErrorCode, r.ExitCode, r.Stderr)
				}
				if r.ExitCode == 0 && r.Parsed != nil {
					commands = append(commands, *r.Parsed)
				}
			}

			answers := map[string]string{}
			for k, v := range s.Challenge.Solution.Answers {
				answers[k] = v
			}
			for _, g := range s.Challenge.Goals {
				if g.Type == "answer" && g.Expected != nil && g.Expected.Static == nil {
					want, err := ExpectedAnswer(env.State, g)
					if err != nil {
						t.Errorf("goal %s: %v", g.ID, err)
					}
					answers[g.ID] = want
				}
			}
			results := Evaluate(env.State, s.Challenge.Goals, Inputs{Answers: answers, Commands: commands})
			var failed []string
			for _, r := range results {
				if !r.Passed {
					failed = append(failed, fmt.Sprintf("%s (%s)", r.GoalID, r.Message))
				}
			}
			if len(failed) > 0 {
				t.Errorf("the reference solution fails: %s", strings.Join(failed, "; "))
			}
		})
	}
}

func onlyAnswerGoals(goals []Goal) bool {
	for _, g := range goals {
		if g.Type != "answer" {
			return false
		}
	}
	return true
}
