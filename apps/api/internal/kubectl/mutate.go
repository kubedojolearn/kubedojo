package kubectl

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// Mutating commands run against a clone of the lab's state. The clone replaces the state only
// if the command changed something and the controllers then reconciled without error, so a
// failed command leaves no trace (simulator-compatibility.md §3, ADR-0006).

// CodeConflict reports a create of something that already exists.
const CodeConflict ErrorCode = "CONFLICT"

// runMutating runs a command that may change the cluster.
func (inv *Invocation) runMutating(env Env, p *Parsed) Result {
	work := env.State.Clone()
	// Changing the kubeconfig isn't a cluster change, so it doesn't move the clock.
	if inv.verb != "config" {
		work.Advance(simulator.Tick)
	}
	wenv := Env{State: work, Events: env.Events, Files: env.Files}
	run := map[string]runner{
		"create": runCreate, "run": runRun, "expose": runExpose, "delete": runDelete, "scale": runScale,
		"set": runSet, "label": runLabel, "annotate": runAnnotate, "rollout": runRollout, "config": runConfig,
		"apply": runApply, "edit": runEdit, "patch": runPatch, "replace": runReplace,
	}[inv.verb]
	res, f := run(inv.args, wenv, p)
	if f != nil {
		r := failed(f, p)
		r.Stdout, r.Redacted, r.Mutated, r.Err = res.Stdout, res.Redacted, res.Mutated, res.Err
		r.Stderr = res.Stderr + r.Stderr
		res = r
	}
	if res.Err != nil || !res.Mutated {
		res.Mutated = false
		return res
	}
	if _, err := simulator.Reconcile(work); err != nil {
		if errors.Is(err, simulator.ErrLimit) {
			// Nothing is kept: the whole command is refused.
			return failed(limitFailure(err), p)
		}
		return Result{Err: err, Parsed: p}
	}
	*env.State = *work
	return res
}

func limitFailure(err error) *Failure {
	return reject(CodeValidation, "error: %s\nⓘ Labs have fixed limits (see the support matrix). Delete resources you no longer need and try again.", err.Error())
}

// writeFailure turns a refused write into kubectl's wording. Errors that aren't the learner's
// (simulator bugs) are returned as err.
func writeFailure(err error) (*Failure, error) {
	var (
		notFound    *simulator.NotFoundError
		exists      *simulator.AlreadyExistsError
		forbidden   *simulator.ForbiddenError
		invalid     *simulator.ValidationError
		unsupported *simulator.UnsupportedError
	)
	switch {
	case errors.As(err, &notFound):
		return reject(CodeNotFound, "Error from server (NotFound): %s %q not found", typeOfKind(notFound.Kind).display(), notFound.Name), nil
	case errors.As(err, &exists):
		return reject(CodeConflict, "Error from server (AlreadyExists): %s %q already exists", typeOfKind(exists.Kind).display(), exists.Name), nil
	case errors.As(err, &forbidden):
		return reject(CodeValidation, "Error from server (Forbidden): %s %q is forbidden: %s", typeOfKind(forbidden.Kind).display(), forbidden.Name, forbidden.Reason), nil
	case errors.As(err, &invalid):
		return reject(CodeValidation, "%s", invalidMessage(invalid)), nil
	case errors.As(err, &unsupported):
		return reject(CodeUnsupported, "error: %s", unsupported.Error()), nil
	case errors.Is(err, simulator.ErrLimit):
		return limitFailure(err), nil
	}
	return nil, err
}

// invalidMessage prints a validation error like kubectl: one reason inline, several as a list.
func invalidMessage(e *simulator.ValidationError) string {
	head := fmt.Sprintf("The %s %q is invalid: ", e.Kind, e.Name)
	if len(e.Reasons) == 1 {
		return head + e.Reasons[0]
	}
	return head + "\n* " + strings.Join(e.Reasons, "\n* ")
}

// outcome collects per-object results of a command that acts on several objects: kubectl
// reports each one and exits 1 if any failed.
type outcome struct {
	stdout, stderr strings.Builder
	failure        *Failure
	mutated        bool
	err            error
}

func (o *outcome) ok(format string, a ...any) { fmt.Fprintf(&o.stdout, format+"\n", a...) }

// fail records a refused write (or a simulator bug in o.err).
func (o *outcome) fail(err error) {
	f, internal := writeFailure(err)
	if internal != nil {
		o.err = internal
		return
	}
	o.failed(f)
}

func (o *outcome) failed(f *Failure) {
	o.stderr.WriteString(f.Message + "\n")
	o.failure = f
}

func (o *outcome) result() (Result, *Failure) {
	res := Result{Stdout: o.stdout.String(), Stderr: o.stderr.String(), Mutated: o.mutated, Err: o.err}
	if o.failure == nil {
		return res, nil
	}
	// The messages are already in stderr; report the code and exit status only.
	res.ExitCode, res.ErrorCode = 1, o.failure.Code
	return res, nil
}

// ---- dry runs and printing created objects ----

var dryRunFlags = []flagDef{
	{Long: "dry-run", Value: true},
	{Long: "output", Short: "o", Value: true},
}

// dryRun reads --dry-run: none (the default), client (print, don't create) or server
// (unsupported: there's no API server to ask).
func dryRun(fl flags) (bool, *Failure) {
	if _, set := fl.values["dry-run"]; !set {
		return false, nil
	}
	switch v := fl.str("dry-run"); v {
	case "none":
		return false, nil
	case "client":
		return true, nil
	case "server":
		return false, reject(CodeUnsupported, "error: --dry-run=server isn't supported: there is no API server to ask; use --dry-run=client")
	default:
		return false, reject(CodeValidation, "error: Invalid dry-run value (%s). Must be \"none\", \"server\", or \"client\".", v)
	}
}

// manifest is an object as the learner would write it: no server-owned metadata or status.
func manifest(o simulator.Object) (map[string]any, error) {
	simulator.TypeMetaFor(o)
	raw, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if meta, ok := m["metadata"].(map[string]any); ok {
		for _, k := range []string{"uid", "resourceVersion", "creationTimestamp", "generation"} {
			delete(meta, k)
		}
	}
	m["status"] = map[string]any{}
	if o.GetKind() == simulator.KindNamespace || o.GetKind() == simulator.KindConfigMap || o.GetKind() == simulator.KindSecret {
		delete(m, "status")
	}
	return m, nil
}

// printObject prints an object for -o yaml|json|name, with Secret values redacted in the copy
// kept in history. For a dry run the object is printed as a manifest.
func printObject(o simulator.Object, output string, dry bool) (Result, *Failure) {
	t := typeOfKind(o.GetKind())
	if output == "name" {
		return Result{Stdout: t.qualified() + "/" + o.Meta().Name + "\n"}, nil
	}
	gen := func() (any, error) {
		if dry {
			return manifest(o.DeepCopy())
		}
		return toGeneric(o)
	}
	plain, err := gen()
	if err != nil {
		return Result{}, reject(CodeValidation, "error: %s", err.Error())
	}
	red, _ := gen()
	if o.GetKind() == simulator.KindSecret {
		redactSecret(red)
	}
	enc := marshalYAML
	if output == "json" {
		enc = marshalJSON
	}
	out, err := enc(plain)
	if err != nil {
		return Result{}, reject(CodeValidation, "error: %s", err.Error())
	}
	redOut, err := enc(red)
	if err != nil {
		return Result{}, reject(CodeValidation, "error: %s", err.Error())
	}
	return Result{Stdout: out, Redacted: redOut}, nil
}

// created reports a create the way kubectl does, honoring --dry-run and -o. verb is
// "created" or "exposed".
func created(env Env, o simulator.Object, verb, output string, dry bool) (Result, *Failure) {
	if f := checkOutput(output, "json", "yaml", "name"); f != nil {
		return Result{}, f
	}
	t := typeOfKind(o.GetKind())
	if dry {
		if output != "" {
			return printObject(o, output, true)
		}
		return Result{Stdout: fmt.Sprintf("%s/%s %s (dry run)\n", t.qualified(), o.Meta().Name, verb)}, nil
	}
	if err := env.State.Create(o); err != nil {
		f, internal := writeFailure(err)
		return Result{Err: internal}, f
	}
	if output != "" {
		stored := env.State.Get(o.GetKind(), o.Meta().Namespace, o.Meta().Name)
		res, f := printObject(stored, output, false)
		res.Mutated = true
		return res, f
	}
	return Result{Stdout: fmt.Sprintf("%s/%s %s\n", t.qualified(), o.Meta().Name, verb), Mutated: true}, nil
}

// ---- parsing helpers ----

// keyValues parses repeated K=V arguments (--from-literal, --env, --labels).
func keyValues(values []string, what string) (map[string]string, []string, *Failure) {
	out := map[string]string{}
	var order []string
	for _, kv := range values {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return nil, nil, reject(CodeValidation, "error: invalid %s %q, expected key=value", what, kv)
		}
		if _, dup := out[k]; !dup {
			order = append(order, k)
		}
		out[k] = v
	}
	return out, order, nil
}

// splitLabels reads --labels=a=b,c=d.
func splitLabels(s string) (map[string]string, *Failure) {
	if s == "" {
		return nil, nil
	}
	m, _, f := keyValues(strings.Split(s, ","), "label spec")
	if f != nil {
		return nil, f
	}
	if reasons := simulator.ValidateLabels(m); len(reasons) > 0 {
		return nil, reject(CodeValidation, "error: %s", strings.Join(reasons, "; "))
	}
	return m, nil
}

// containerName derives a container name from an image the way `kubectl create deployment`
// does: the last path element without tag or digest, made DNS-label safe.
func containerName(image string) string {
	name := image
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	name, _, _ = strings.Cut(name, "@")
	name, _, _ = strings.Cut(name, ":")
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "container"
	}
	return out
}

// requireName checks the one NAME argument of a create-style command.
func requireName(pos []string, usage string) (string, *Failure) {
	if len(pos) == 0 {
		return "", reject(CodeValidation, "error: NAME is required\nUsage: %s", usage)
	}
	if len(pos) > 1 {
		return "", reject(CodeValidation, "error: exactly one NAME is required, got %d\nUsage: %s", len(pos), usage)
	}
	return pos[0], nil
}

// writableTargets resolves TYPE NAME… / TYPE/NAME… / TYPE -l / TYPE --all into objects.
// Missing names are reported in stderr (and as a failure) but don't stop the others.
func writableTargets(env Env, pos []string, fl flags, verb string, o *outcome) ([]simulator.Object, *Failure) {
	targets, f := resolveTargets(pos, verb)
	if f != nil {
		return nil, f
	}
	sel, err := simulator.ParseSelector(fl.str("selector"))
	if err != nil {
		return nil, reject(CodeValidation, "error: %s", err.Error())
	}
	ns, all := namespaceFor(fl, env), fl.boolean("all-namespaces")
	byName := false
	for _, tg := range targets {
		if len(tg.names) > 0 {
			byName = true
		}
		if tg.t.Kind == kindEvent || tg.t.Kind == kindEndpoints {
			return nil, reject(CodeValidation, "error: %s are maintained by the simulator and can't be changed with kubectl %s", tg.t.Plural, verb)
		}
	}
	if byName && (fl.str("selector") != "" || fl.boolean("all")) {
		return nil, reject(CodeValidation, "error: name cannot be provided when a selector or --all is specified")
	}
	if byName && all {
		return nil, reject(CodeValidation, "error: a resource cannot be retrieved by name across all namespaces")
	}
	if !byName && fl.str("selector") == "" && !fl.boolean("all") {
		return nil, reject(CodeValidation, "error: resource(s) were provided, but no name was specified")
	}
	var out []simulator.Object
	for _, tg := range targets {
		if len(tg.names) == 0 {
			for _, it := range listItems(env, tg.t, ns, all) {
				if sel.Matches(it.labels) {
					out = append(out, it.obj)
				}
			}
			continue
		}
		for _, name := range tg.names {
			obj := env.State.Get(tg.t.Kind, ns, name)
			if obj == nil {
				o.failed(reject(CodeNotFound, "Error from server (NotFound): %s %q not found", tg.t.display(), name))
				continue
			}
			out = append(out, obj)
		}
	}
	return out, nil
}

// noteParsed records the objects a command acted on for observation goals.
func noteParsed(p *Parsed, ns string, objs []simulator.Object) {
	p.Namespace = ns
	seen := map[string]bool{}
	for _, o := range objs {
		if !seen[o.GetKind()] {
			seen[o.GetKind()] = true
			p.Resources = append(p.Resources, o.GetKind())
		}
		p.Names = append(p.Names, o.Meta().Name)
	}
}
