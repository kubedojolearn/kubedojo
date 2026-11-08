package kubectl

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// Env is what a command can see: the lab's state and its stored event history.
type Env struct {
	State *simulator.State
	// Events are the lab's persisted events, oldest first (the state only holds new ones).
	Events []simulator.Event
	// Files are the lab editor's virtual files by path (manifests/web.yaml), read by -f.
	Files map[string]string
}

// Parsed is the structured form of a command, stored with its execution for observation goals.
type Parsed struct {
	Verb          string   `json:"verb"`
	Subcommand    string   `json:"subcommand,omitempty"`
	Resources     []string `json:"resources,omitempty"`
	Names         []string `json:"names,omitempty"`
	Namespace     string   `json:"namespace,omitempty"`
	AllNamespaces bool     `json:"allNamespaces,omitempty"`
	Output        string   `json:"output,omitempty"`
	Selector      string   `json:"selector,omitempty"`
	Flags         []string `json:"flags,omitempty"`
}

// Result is a command's terminal output.
type Result struct {
	Stdout    string
	Stderr    string
	ExitCode  int
	ErrorCode ErrorCode // "" unless the command failed in a way the UI highlights
	// Redacted is Stdout with Secret values replaced; it is what command history stores.
	Redacted string
	// Parsed is nil when the input was rejected before parsing.
	Parsed *Parsed
	// Clear asks the terminal to clear its screen (the `clear` command).
	Clear bool
	// Mutated is set when the command changed the lab (cluster or kubeconfig). The state passed
	// in has then been replaced by the reconciled result, which the caller must persist.
	Mutated bool
	// Err is a simulator bug (never a learner error): the caller must discard the state and
	// report an internal error.
	Err error
}

func failed(f *Failure, p *Parsed) Result {
	return Result{Stderr: f.Message + "\n", ExitCode: f.Exit, ErrorCode: f.Code, Parsed: p}
}

// Invocation is a tokenized, classified command, ready to run.
type Invocation struct {
	program string
	verb    string
	args    []string
	// Mutating commands need the write path (advance the clock, reconcile, persist).
	Mutating bool
	failure  *Failure
}

// readVerbs are the commands this simulator version runs (simulator-compatibility.md §3).
var readVerbs = map[string]bool{
	"get": true, "describe": true, "logs": true, "events": true, "explain": true, "api-resources": true,
	"api-versions": true, "cluster-info": true, "version": true, "config": true, "rollout": true, "help": true,
}

// mutatingVerbs are kubectl commands that may change the cluster. They run against a copy of
// the state that replaces it only on success. `rollout undo|restart` and `config set-context`
// are mutating too (see mutates).
var mutatingVerbs = map[string]bool{
	"create": true, "apply": true, "delete": true, "run": true, "expose": true, "scale": true, "set": true,
	"label": true, "annotate": true, "edit": true, "patch": true, "replace": true,
}

// mutates reports whether a verb with these arguments may change the lab.
func mutates(verb string, args []string) bool {
	if mutatingVerbs[verb] {
		return true
	}
	sub := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "-n" || args[i] == "--namespace" {
			i++
			continue
		}
		if !strings.HasPrefix(args[i], "-") {
			sub = args[i]
			break
		}
	}
	switch verb {
	case "rollout":
		return sub == "undo" || sub == "restart"
	case "config":
		return sub == "set-context"
	}
	return false
}

// otherVerbs are real kubectl commands the simulator doesn't run, with a reason.
var otherVerbs = map[string]string{
	"exec":         "no containers actually run in the simulator, so there is nothing to exec into",
	"attach":       "no containers actually run in the simulator, so there is nothing to attach to",
	"port-forward": "no containers actually run in the simulator, so there is no port to forward",
	"proxy":        "the simulator has no API server to proxy",
	"cp":           "no containers actually run in the simulator, so there are no files to copy",
	"debug":        "no containers actually run in the simulator",
	"top":          "the simulator has no metrics server",
	"auth":         "the simulator has a single user with full access",
	"certificate":  "the simulator has no certificate signing",
	"cordon":       "node maintenance isn't simulated yet",
	"uncordon":     "node maintenance isn't simulated yet",
	"drain":        "node maintenance isn't simulated yet",
	"taint":        "changing node taints isn't simulated yet",
	"wait":         "the simulated cluster is fully reconciled after every command, so there is nothing to wait for",
	"diff":         "diff isn't simulated; compare with 'kubectl get ... -o yaml'",
	"autoscale":    "the simulator has no autoscaler",
	"plugin":       "plugins can't be installed in the simulator",
	"kustomize":    "kustomize isn't part of the simulator",
	"completion":   "shell completion isn't needed in the browser terminal",
	"alpha":        "alpha commands aren't simulated",
}

// Plan tokenizes and classifies input without touching any state.
func Plan(input string) *Invocation {
	tokens, f := Tokenize(input)
	if f != nil {
		return &Invocation{failure: f}
	}
	if len(tokens) == 0 {
		return &Invocation{}
	}
	inv := &Invocation{program: tokens[0]}
	switch tokens[0] {
	case "kubectl", "k":
	case "help", "clear":
		return inv
	default:
		inv.failure = &Failure{
			Code:    CodeUnsupported,
			Message: fmt.Sprintf("%s: command not available. Only kubectl commands run in the simulator (try 'help').", truncateWord(tokens[0])),
			Exit:    127,
		}
		return inv
	}
	rest := tokens[1:]
	// The verb is the first word that isn't a global flag or its value.
	for i := 0; i < len(rest); i++ {
		a := rest[i]
		if a == "-n" || a == "--namespace" {
			i++
			continue
		}
		if strings.HasPrefix(a, "-") {
			continue
		}
		inv.verb = a
		inv.args = append(append([]string{}, rest[:i]...), rest[i+1:]...)
		break
	}
	if inv.verb == "" {
		inv.args = rest
	}
	inv.Mutating = mutates(inv.verb, inv.args)
	return inv
}

func truncateWord(s string) string {
	if len(s) > 40 {
		return s[:40] + "…"
	}
	return s
}

// Run executes the invocation. Read commands never change env.State; mutating commands
// replace it with the reconciled result when they succeed (Result.Mutated).
func (inv *Invocation) Run(env Env) Result {
	if inv.failure != nil {
		return failed(inv.failure, nil)
	}
	switch inv.program {
	case "":
		return Result{Parsed: &Parsed{}}
	case "help":
		return Result{Stdout: helpText, Parsed: &Parsed{Verb: "help"}}
	case "clear":
		return Result{Clear: true, Parsed: &Parsed{Verb: "clear"}}
	}
	if inv.verb == "" {
		_, fl, f := parseFlags(inv.args, "", nil)
		if f != nil {
			return failed(f, &Parsed{})
		}
		_ = fl
		return Result{Stdout: helpText, Parsed: &Parsed{Verb: "help"}}
	}
	p := &Parsed{Verb: inv.verb}
	var res Result
	switch {
	case inv.Mutating:
		res = inv.runMutating(env, p)
	case readVerbs[inv.verb]:
		res = runRead(inv.verb, inv.args, env, p)
	case otherVerbs[inv.verb] != "":
		return failed(reject(CodeUnsupported, "error: 'kubectl %s' isn't supported: %s.", inv.verb, otherVerbs[inv.verb]), p)
	default:
		return failed(reject(CodeUnsupported, "error: unknown command %q for \"kubectl\"\nRun 'kubectl help' for the commands the simulator supports.", truncateWord(inv.verb)), p)
	}
	if res.Parsed == nil {
		res.Parsed = p
	}
	if res.Redacted == "" {
		res.Redacted = res.Stdout
	}
	return res
}

// Execute plans and runs input in one step.
func Execute(input string, env Env) Result {
	return Plan(input).Run(env)
}

type runner func(args []string, env Env, p *Parsed) (Result, *Failure)

func runRead(verb string, args []string, env Env, p *Parsed) Result {
	run := map[string]runner{
		"get": runGet, "describe": runDescribe, "logs": runLogs, "events": runEvents, "explain": runExplain,
		"api-resources": runAPIResources, "api-versions": runAPIVersions, "cluster-info": runClusterInfo,
		"version": runVersion, "config": runConfig, "rollout": runRollout, "help": runHelp,
	}[verb]
	res, f := run(args, env, p)
	if f != nil {
		r := failed(f, p)
		// Partial output (e.g. some names found) is kept alongside the error.
		r.Stdout, r.Redacted = res.Stdout, res.Redacted
		r.Stderr = res.Stderr + r.Stderr
		return r
	}
	return res
}

// namespaceFor returns the namespace a command operates in: -n, or the kubeconfig's current one.
func namespaceFor(fl flags, env Env) string {
	if ns := fl.str("namespace"); ns != "" {
		return ns
	}
	if env.State.CurrentNamespace != "" {
		return env.State.CurrentNamespace
	}
	return "default"
}

func recordFlags(p *Parsed, fl flags) {
	seen := map[string]bool{}
	for _, name := range fl.order {
		if !seen[name] {
			seen[name] = true
			p.Flags = append(p.Flags, name)
		}
	}
	sort.Strings(p.Flags)
}

func runHelp(args []string, _ Env, p *Parsed) (Result, *Failure) {
	if len(args) > 0 {
		if h, ok := commandHelp[args[0]]; ok {
			return Result{Stdout: h}, nil
		}
	}
	return Result{Stdout: helpText}, nil
}
