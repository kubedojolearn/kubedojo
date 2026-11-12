// Package grading decides whether a lab's goals pass (ADR-0007). It is pure: it reads the
// simulated cluster, the learner's submitted answers and the structured form of the commands
// they ran, and never a raw command string. Expected values are computed here, on the server,
// and never leave it; failure messages come from the lesson and never reveal them.
package grading

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/kubectl"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// Goal is one goal of a challenge, as authored (packages/content-schema challenge.schema.json).
type Goal struct {
	ID             string      `json:"id"`
	Type           string      `json:"type"`
	Description    string      `json:"description"`
	FailureMessage string      `json:"failureMessage"`
	Target         *Target     `json:"target"`
	Absent         bool        `json:"absent"`
	Assertions     []Assertion `json:"assertions"`
	Prompt         string      `json:"prompt"`
	Input          string      `json:"input"`
	CaseSensitive  *bool       `json:"caseSensitive"`
	Choices        []string    `json:"choices"`
	Expected       *Expected   `json:"expected"`
	Command        *Command    `json:"command"`
}

// Target names one object, by name or by a label selector that must match exactly one.
type Target struct {
	Kind          string `json:"kind"`
	Namespace     string `json:"namespace"`
	Name          string `json:"name"`
	LabelSelector string `json:"labelSelector"`
}

// Assertion compares the value at Path with Value.
type Assertion struct {
	Path     string `json:"path"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
}

// Expected is how an answer goal's answer is computed: exactly one field is set.
type Expected struct {
	Static any `json:"static"`
	Path   *struct {
		Target Target `json:"target"`
		Path   string `json:"path"`
	} `json:"path"`
	Count *struct {
		Kind          string `json:"kind"`
		Namespace     string `json:"namespace"`
		LabelSelector string `json:"labelSelector"`
	} `json:"count"`
}

// Command is the parsed shape an observation goal looks for.
type Command struct {
	Verb          string `json:"verb"`
	Subverb       string `json:"subverb"`
	Kind          string `json:"kind"`
	Namespace     string `json:"namespace"`
	Name          string `json:"name"`
	Output        string `json:"output"`
	SetsNamespace string `json:"setsNamespace"`
}

// Result is one goal's outcome, as shown to the learner.
type Result struct {
	GoalID  string `json:"goalId"`
	Passed  bool   `json:"passed"`
	Message string `json:"message"`
}

// Inputs are what the learner did besides changing the cluster.
type Inputs struct {
	// Answers holds the latest submitted answer per answer goal.
	Answers map[string]string
	// Commands are the parsed forms of the commands that ran successfully (exit code 0).
	Commands []kubectl.Parsed
}

// MaxSubmissions is how many answers a goal accepts per lab (ADR-0007): 10, and for choice
// goals one fewer than the number of choices, so they can't be passed by trying every one.
func MaxSubmissions(g Goal) int {
	if g.Input == "choice" {
		return min(10, max(len(g.Choices)-1, 1))
	}
	return 10
}

// Evaluate grades every goal against the state.
func Evaluate(state *simulator.State, goals []Goal, in Inputs) []Result {
	out := make([]Result, 0, len(goals))
	for _, g := range goals {
		passed, err := evaluate(state, g, in)
		r := Result{GoalID: g.ID, Passed: passed && err == nil}
		switch {
		case r.Passed:
			r.Message = "Done."
		case g.FailureMessage != "":
			r.Message = g.FailureMessage
		case g.Type == "answer" && in.Answers[g.ID] == "":
			r.Message = "Answer the question above, then check again."
		case g.Type == "answer":
			r.Message = "That answer isn't right yet. Look again in the cluster."
		case g.Type == "observation":
			r.Message = "Run the command this goal describes, then check again."
		default:
			r.Message = "Not done yet. Compare the cluster with what the goal asks for."
		}
		out = append(out, r)
	}
	return out
}

// Score is the percentage of goals that passed.
func Score(results []Result) int {
	if len(results) == 0 {
		return 0
	}
	n := 0
	for _, r := range results {
		if r.Passed {
			n++
		}
	}
	return n * 100 / len(results)
}

func evaluate(state *simulator.State, g Goal, in Inputs) (bool, error) {
	switch g.Type {
	case "state":
		if g.Target == nil {
			return false, fmt.Errorf("goal %s has no target", g.ID)
		}
		objs, err := find(state, *g.Target)
		if err != nil {
			return false, err
		}
		if g.Absent {
			return len(objs) == 0, nil
		}
		if len(objs) != 1 {
			return false, nil
		}
		doc, err := toGeneric(objs[0])
		if err != nil {
			return false, err
		}
		for _, a := range g.Assertions {
			ok, err := Check(doc, a)
			if err != nil || !ok {
				return false, err
			}
		}
		return true, nil
	case "answer":
		given, ok := in.Answers[g.ID]
		if !ok {
			return false, nil
		}
		want, err := ExpectedAnswer(state, g)
		if err != nil {
			return false, err
		}
		return AnswerMatches(g, want, given), nil
	case "observation":
		if g.Command == nil {
			return false, fmt.Errorf("goal %s has no command", g.ID)
		}
		for _, p := range in.Commands {
			if Observed(*g.Command, p) {
				return true, nil
			}
		}
		return false, nil
	}
	return false, fmt.Errorf("goal %s has unknown type %q", g.ID, g.Type)
}

// find returns the objects a target names: one by name, or every match of the selector.
func find(state *simulator.State, t Target) ([]simulator.Object, error) {
	if !simulator.SupportedKind(t.Kind) {
		return nil, fmt.Errorf("unsupported kind %q", t.Kind)
	}
	ns := t.Namespace
	if ns == "" && !simulator.ClusterScoped(t.Kind) {
		ns = "default"
	}
	if t.Name != "" {
		if o := state.Get(t.Kind, ns, t.Name); o != nil {
			return []simulator.Object{o}, nil
		}
		return nil, nil
	}
	sel, err := simulator.ParseSelector(t.LabelSelector)
	if err != nil {
		return nil, err
	}
	var out []simulator.Object
	for _, o := range state.List(t.Kind, ns) {
		if sel.Matches(o.Meta().Labels) {
			out = append(out, o)
		}
	}
	return out, nil
}

func toGeneric(o simulator.Object) (any, error) {
	b, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	var v any
	return v, json.Unmarshal(b, &v)
}

// ExpectedAnswer computes an answer goal's expected value from the current state.
func ExpectedAnswer(state *simulator.State, g Goal) (string, error) {
	e := g.Expected
	switch {
	case e == nil:
		return "", fmt.Errorf("goal %s has no expected answer", g.ID)
	case e.Static != nil:
		return scalarString(e.Static), nil
	case e.Path != nil:
		objs, err := find(state, e.Path.Target)
		if err != nil {
			return "", err
		}
		if len(objs) != 1 {
			return "", fmt.Errorf("goal %s: target matches %d objects", g.ID, len(objs))
		}
		doc, err := toGeneric(objs[0])
		if err != nil {
			return "", err
		}
		v, ok, err := Lookup(doc, e.Path.Path)
		if err != nil || !ok || v == nil {
			return "", fmt.Errorf("goal %s: %s has no value", g.ID, e.Path.Path)
		}
		return scalarString(v), nil
	case e.Count != nil:
		objs, err := find(state, Target{Kind: e.Count.Kind, Namespace: e.Count.Namespace, LabelSelector: e.Count.LabelSelector})
		if err != nil {
			return "", err
		}
		return strconv.Itoa(len(objs)), nil
	}
	return "", fmt.Errorf("goal %s has an empty expected answer", g.ID)
}

// AnswerMatches compares a submitted answer with the expected one: surrounding whitespace is
// ignored, numbers compare by value, and text compares case-insensitively if the goal says so.
func AnswerMatches(g Goal, want, given string) bool {
	given, want = strings.TrimSpace(given), strings.TrimSpace(want)
	if g.Input == "number" {
		a, err1 := strconv.ParseFloat(given, 64)
		b, err2 := strconv.ParseFloat(want, 64)
		return err1 == nil && err2 == nil && a == b
	}
	if g.Input == "text" && g.CaseSensitive != nil && !*g.CaseSensitive {
		return strings.EqualFold(given, want)
	}
	return given == want
}

// Observed reports whether a parsed command has the shape an observation goal asks for.
func Observed(c Command, p kubectl.Parsed) bool {
	if p.Verb != c.Verb {
		return false
	}
	if c.Subverb != "" && p.Subcommand != c.Subverb {
		return false
	}
	if c.Kind != "" && !contains(p.Resources, c.Kind) {
		return false
	}
	if c.Name != "" && !contains(p.Names, c.Name) {
		return false
	}
	if c.Namespace != "" && !p.AllNamespaces && p.Namespace != c.Namespace {
		return false
	}
	if c.Output != "" {
		got := p.Output
		if got == "" {
			got = "table"
		}
		if got != c.Output {
			return false
		}
	}
	if c.SetsNamespace != "" && (p.Subcommand != "set-context" || p.Namespace != c.SetsNamespace) {
		return false
	}
	return true
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func scalarString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == math.Trunc(x) && math.Abs(x) < 1e15 {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int:
		return strconv.Itoa(x)
	case bool:
		return strconv.FormatBool(x)
	}
	b, _ := json.Marshal(v)
	return string(b)
}
