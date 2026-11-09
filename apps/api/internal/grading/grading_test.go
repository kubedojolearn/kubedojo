package grading

import (
	"encoding/json"
	"testing"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/kubectl"
)

const doc = `{
  "metadata": {"name": "web", "labels": {"app": "web"}, "annotations": {"deployment.kubernetes.io/revision": "3"}},
  "spec": {"replicas": 3, "template": {"spec": {"containers": [
    {"name": "web", "image": "nginx:1.27", "resources": {"requests": {"cpu": "500m", "memory": "256Mi"}}}
  ]}}},
  "status": {"readyReplicas": 3},
  "endpoints": [{"addresses": ["10.244.1.2"]}, {"addresses": ["10.244.2.2"]}]
}`

func TestAssertions(t *testing.T) {
	var d any
	if err := json.Unmarshal([]byte(doc), &d); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		a    Assertion
		want bool
	}{
		{Assertion{"spec.replicas", "equals", 3.0}, true},
		{Assertion{"spec.replicas", "equals", "3"}, true},
		{Assertion{"spec.replicas", "notEquals", 2.0}, true},
		{Assertion{"spec.template.spec.containers[0].image", "equals", "nginx:1.27"}, true},
		{Assertion{"spec.template.spec.containers[1].image", "exists", nil}, false},
		{Assertion{`metadata.annotations["deployment.kubernetes.io/revision"]`, "greaterThanOrEqual", 3.0}, true},
		{Assertion{`metadata.annotations["deployment.kubernetes.io/revision"]`, "greaterThanOrEqual", 4.0}, false},
		{Assertion{"metadata.labels.app", "in", []any{"api", "web"}}, true},
		{Assertion{"metadata.labels.tier", "notExists", nil}, true},
		{Assertion{"metadata.labels.tier", "notEquals", "web"}, true},
		{Assertion{"metadata.labels.tier", "equals", "web"}, false},
		{Assertion{"metadata.name", "contains", "we"}, true},
		{Assertion{"endpoints", "lengthEquals", 2.0}, true},
		{Assertion{"endpoints", "lengthGreaterThanOrEqual", 3.0}, false},
		{Assertion{"endpoints[0].addresses", "contains", "10.244.1.2"}, true},
		{Assertion{"spec.template.spec.containers[0].resources.requests.cpu", "lessThanOrEqual", "1"}, true},
		{Assertion{"spec.template.spec.containers[0].resources.requests.cpu", "equals", "0.5"}, true},
		{Assertion{"spec.template.spec.containers[0].resources.requests.cpu", "greaterThanOrEqual", 1.0}, false},
		{Assertion{"spec.template.spec.containers[0].resources.requests.memory", "lessThanOrEqual", "1Gi"}, true},
		{Assertion{"spec.template.spec.containers[0].resources.requests.memory", "greaterThanOrEqual", "512Mi"}, false},
	}
	for _, c := range cases {
		got, err := Check(d, c.a)
		if err != nil || got != c.want {
			t.Errorf("%s %s %v: got %v (err %v), want %v", c.a.Path, c.a.Operator, c.a.Value, got, err, c.want)
		}
	}
	for _, bad := range []string{"", "a..b", "a[", "a[x]", `a[""]`, "a.[0]"} {
		if _, err := Check(d, Assertion{bad, "exists", nil}); err == nil {
			t.Errorf("path %q accepted", bad)
		}
	}
}

func TestAnswers(t *testing.T) {
	no := false
	cases := []struct {
		g          Goal
		want, give string
		ok         bool
	}{
		{Goal{Input: "number"}, "3", " 3 ", true},
		{Goal{Input: "number"}, "3", "3.0", true},
		{Goal{Input: "number"}, "3", "three", false},
		{Goal{Input: "text"}, "node-2", "Node-2", false},
		{Goal{Input: "text", CaseSensitive: &no}, "node-2", " Node-2", true},
		{Goal{Input: "choice"}, "FailedScheduling", "FailedScheduling", true},
		{Goal{Input: "choice"}, "FailedScheduling", "failedscheduling", false},
	}
	for _, c := range cases {
		if got := AnswerMatches(c.g, c.want, c.give); got != c.ok {
			t.Errorf("%s: %q vs %q = %v", c.g.Input, c.want, c.give, got)
		}
	}
	if MaxSubmissions(Goal{Input: "choice", Choices: []string{"a", "b", "c", "d"}}) != 3 || MaxSubmissions(Goal{Input: "text"}) != 10 {
		t.Fatal("submission limits")
	}
}

func TestObservations(t *testing.T) {
	describe := kubectl.Parsed{Verb: "describe", Resources: []string{"Pod"}, Names: []string{"api"}, Namespace: "default"}
	cases := []struct {
		c    Command
		p    kubectl.Parsed
		want bool
	}{
		{Command{Verb: "describe", Kind: "Pod", Name: "api"}, describe, true},
		{Command{Verb: "describe", Kind: "Pod", Name: "web"}, describe, false},
		{Command{Verb: "describe", Kind: "Pod", Namespace: "shop"}, describe, false},
		{Command{Verb: "get", Kind: "Pod"}, describe, false},
		{Command{Verb: "get", Kind: "Pod", Namespace: "shop"}, kubectl.Parsed{Verb: "get", Resources: []string{"Pod"}, AllNamespaces: true}, true},
		{Command{Verb: "get", Output: "table"}, kubectl.Parsed{Verb: "get"}, true},
		{Command{Verb: "get", Output: "yaml"}, kubectl.Parsed{Verb: "get", Output: "wide"}, false},
		{Command{Verb: "rollout", Subverb: "history"}, kubectl.Parsed{Verb: "rollout", Subcommand: "status"}, false},
		{Command{Verb: "config", SetsNamespace: "shop"}, kubectl.Parsed{Verb: "config", Subcommand: "set-context", Namespace: "shop"}, true},
		{Command{Verb: "config", SetsNamespace: "shop"}, kubectl.Parsed{Verb: "config", Subcommand: "view"}, false},
	}
	for i, c := range cases {
		if got := Observed(c.c, c.p); got != c.want {
			t.Errorf("case %d: %v", i, got)
		}
	}
}

func TestScoreAndMessages(t *testing.T) {
	goals := []Goal{
		{ID: "a", Type: "answer", Input: "number", Expected: &Expected{Static: 2.0}},
		{ID: "b", Type: "answer", Input: "number", Expected: &Expected{Static: 2.0}, FailureMessage: "Count again."},
		{ID: "c", Type: "observation", Command: &Command{Verb: "get"}},
	}
	res := Evaluate(nil, goals, Inputs{Answers: map[string]string{"a": "2", "b": "3"}, Commands: []kubectl.Parsed{{Verb: "get"}}})
	if !res[0].Passed || res[1].Passed || res[1].Message != "Count again." || !res[2].Passed || Score(res) != 66 {
		t.Fatalf("%+v score %d", res, Score(res))
	}
	// The expected answer never appears in feedback.
	for _, r := range Evaluate(nil, goals[:1], Inputs{Answers: map[string]string{"a": "5"}}) {
		if r.Passed || r.Message == "" || containsDigit(r.Message, '2') {
			t.Fatalf("%+v", r)
		}
	}
}

func containsDigit(s string, d rune) bool {
	for _, r := range s {
		if r == d {
			return true
		}
	}
	return false
}
