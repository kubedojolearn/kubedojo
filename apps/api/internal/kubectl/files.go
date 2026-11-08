package kubectl

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// Manifests for create -f, apply -f and delete -f come from the lab editor's virtual files
// (Env.Files), never from the server's file system or the network.

// LastAppliedAnnotation holds the manifest `kubectl apply` last applied, for its three-way merge.
const LastAppliedAnnotation = "kubectl.kubernetes.io/last-applied-configuration"

// fileDir is the one directory of virtual files.
const fileDir = "manifests"

// doc is one object read from a manifest file.
type doc struct {
	path string
	raw  map[string]any // as written, with namespace filled in
	obj  simulator.Object
}

// filePaths expands -f values: virtual file paths, or the manifests directory (every file in
// name order, like kubectl with a directory).
func filePaths(env Env, values []string) ([]string, *Failure) {
	var out []string
	for _, v := range values {
		for _, p := range strings.Split(v, ",") {
			switch {
			case p == "-":
				return nil, reject(CodeUnsupported, "error: reading manifests from stdin isn't supported; save the manifest in the lab editor and pass its path")
			case strings.Contains(p, "://"):
				return nil, reject(CodeUnsupported, "error: the simulator can't fetch manifests from URLs; paste the manifest into the lab editor and pass its path")
			}
			clean := path.Clean(strings.TrimPrefix(p, "./"))
			if clean == fileDir {
				var dir []string
				for name := range env.Files {
					dir = append(dir, name)
				}
				sort.Strings(dir)
				if len(dir) == 0 {
					return nil, reject(CodeNotFound, "error: no objects passed to apply: the lab has no files in %s/", fileDir)
				}
				out = append(out, dir...)
				continue
			}
			if _, ok := env.Files[clean]; !ok {
				hint := ""
				if !strings.HasPrefix(clean, fileDir+"/") {
					hint = fmt.Sprintf("\nⓘ Files from the lab editor live in %s/, e.g. %s/%s", fileDir, fileDir, path.Base(clean))
				} else if len(env.Files) > 0 {
					hint = "\nⓘ The lab's files are: " + strings.Join(sortedKeys(env.Files), ", ")
				}
				return nil, reject(CodeNotFound, "error: the path %q does not exist%s", p, hint)
			}
			out = append(out, clean)
		}
	}
	return out, nil
}

// readDocs parses the files into objects. Namespaced objects without a namespace get ns; an
// object whose namespace differs from an explicit -n is refused, as kubectl does.
func readDocs(env Env, paths []string, ns string, explicitNS bool) ([]doc, *Failure) {
	var out []doc
	for _, p := range paths {
		dec := yaml.NewDecoder(strings.NewReader(env.Files[p]))
		for {
			var v any
			err := dec.Decode(&v)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, reject(CodeValidation, "error: error parsing %s: error converting YAML to JSON: %s", p, err.Error())
			}
			if v == nil {
				continue
			}
			m, ok := jsonValue(v).(map[string]any)
			if !ok {
				return nil, reject(CodeValidation, "error: error parsing %s: each YAML document must be an object with apiVersion and kind", p)
			}
			items := []any{m}
			if m["kind"] == "List" {
				list, _ := m["items"].([]any)
				items = list
			}
			for _, it := range items {
				im, ok := it.(map[string]any)
				if !ok {
					return nil, reject(CodeValidation, "error: error parsing %s: List items must be objects", p)
				}
				d, f := toDoc(p, im, ns, explicitNS)
				if f != nil {
					return nil, f
				}
				out = append(out, d)
			}
		}
	}
	if len(out) == 0 {
		return nil, reject(CodeValidation, "error: no objects passed to apply")
	}
	return out, nil
}

func toDoc(p string, m map[string]any, ns string, explicitNS bool) (doc, *Failure) {
	if m["apiVersion"] == nil || m["kind"] == nil {
		return doc{}, reject(CodeValidation, "error: error validating %q: error validating data: [apiVersion not set, kind not set]: every object needs apiVersion and kind", p)
	}
	kind, _ := m["kind"].(string)
	apiVersion, _ := m["apiVersion"].(string)
	if !simulator.SupportedKind(kind) {
		name := ""
		if meta, ok := m["metadata"].(map[string]any); ok {
			name, _ = meta["name"].(string)
		}
		return doc{}, reject(CodeUnsupported, "error: resource mapping not found for name: %q from %q: no matches for kind %q in version %q\nⓘ %s isn't part of the KubeDojo simulator. Run 'kubectl api-resources' to see what is.", name, p, kind, apiVersion, kind)
	}
	meta, _ := m["metadata"].(map[string]any)
	if meta == nil {
		meta = map[string]any{}
		m["metadata"] = meta
	}
	if simulator.ClusterScoped(kind) {
		delete(meta, "namespace")
	} else {
		own, _ := meta["namespace"].(string)
		switch {
		case own == "":
			meta["namespace"] = ns
		case explicitNS && own != ns:
			return doc{}, reject(CodeValidation, "error: the namespace from the provided object %q does not match the namespace %q. You must pass '--namespace=%s' to perform this operation.", own, ns, own)
		}
	}
	raw, err := json.Marshal(m)
	if err != nil {
		return doc{}, reject(CodeValidation, "error: error parsing %s: %s", p, err.Error())
	}
	obj, err := simulator.DecodeInput(raw)
	if err != nil {
		var unsupported *simulator.UnsupportedError
		if errors.As(err, &unsupported) {
			return doc{}, reject(CodeUnsupported, "error: error validating %q: %s", p, err.Error())
		}
		return doc{}, reject(CodeValidation, "error: error validating %q: %s", p, err.Error())
	}
	return doc{path: p, raw: m, obj: obj}, nil
}

// jsonValue turns decoded YAML into JSON-compatible values (string map keys, float numbers).
func jsonValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			x[k] = jsonValue(e)
		}
		return x
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, e := range x {
			out[fmt.Sprint(k)] = jsonValue(e)
		}
		return out
	case []any:
		for i, e := range x {
			x[i] = jsonValue(e)
		}
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case uint64:
		return float64(x)
	}
	return v
}

// fileFailure rewords a refused write the way kubectl reports errors for objects from files.
func fileFailure(f *Failure, action, p string) *Failure {
	if rest, ok := strings.CutPrefix(f.Message, "Error from server ("); ok {
		if reason, msg, ok := strings.Cut(rest, "): "); ok {
			f.Message = fmt.Sprintf("Error from server (%s): error when %s %q: %s", reason, action, p, msg)
			return f
		}
	}
	f.Message = fmt.Sprintf("error: error when %s %q: %s", action, p, strings.TrimPrefix(f.Message, "error: "))
	return f
}

// fileTarget reads -f, --dry-run and -o for the file commands.
func fileTargets(fl flags, env Env, p *Parsed) ([]doc, bool, *Failure) {
	dry, f := dryRun(fl)
	if f != nil {
		return nil, false, f
	}
	if f := checkOutput(fl.str("output"), "name"); f != nil {
		return nil, false, f
	}
	paths, f := filePaths(env, fl.values["filename"])
	if f != nil {
		return nil, false, f
	}
	ns := namespaceFor(fl, env)
	docs, f := readDocs(env, paths, ns, fl.str("namespace") != "")
	if f != nil {
		return nil, false, f
	}
	var objs []simulator.Object
	for _, d := range docs {
		objs = append(objs, d.obj)
	}
	noteParsed(p, ns, objs)
	return docs, dry, nil
}

// report prints one object's outcome: kubectl's "kind/name verb", or kind/name for -o name.
func report(o *outcome, obj simulator.Object, verb string, dry bool, output string) {
	t := typeOfKind(obj.GetKind())
	if output == "name" {
		o.ok("%s/%s", t.qualified(), obj.Meta().Name)
		return
	}
	suffix := ""
	if dry {
		suffix = " (dry run)"
	}
	o.ok("%s/%s %s%s", t.qualified(), obj.Meta().Name, verb, suffix)
}

// ---- create -f ----

func createFromFile(fl flags, env Env, p *Parsed, pos []string) (Result, *Failure) {
	if len(pos) > 0 {
		return Result{}, reject(CodeValidation, "error: Unexpected args: %v\nSee 'kubectl create -h' for help and examples", pos)
	}
	docs, dry, f := fileTargets(fl, env, p)
	if f != nil {
		return Result{}, f
	}
	var o outcome
	for _, d := range docs {
		if fl.boolean("save-config") {
			if err := setLastApplied(d); err != nil {
				return Result{Err: err}, nil
			}
		}
		if err := env.State.Create(d.obj); err != nil {
			if fail, internal := writeFailure(err); internal != nil {
				o.err = internal
			} else {
				o.failed(fileFailure(fail, "creating", d.path))
			}
			continue
		}
		o.mutated = true
		report(&o, d.obj, "created", dry, fl.str("output"))
	}
	if dry {
		o.mutated = false // the writes were made on a copy, to check later objects against earlier ones
	}
	return o.result()
}

// ---- delete -f ----

func deleteFromFile(fl flags, env Env, p *Parsed, pos []string) (Result, *Failure) {
	if len(pos) > 0 {
		return Result{}, reject(CodeValidation, "error: when paths, URLs, or stdin is provided as input, you may not specify resource arguments as well")
	}
	if fl.str("selector") != "" || fl.boolean("all") {
		return Result{}, reject(CodeValidation, "error: -l and --all can't be combined with -f")
	}
	paths, f := filePaths(env, fl.values["filename"])
	if f != nil {
		return Result{}, f
	}
	if f := checkOutput(fl.str("output"), "name"); f != nil {
		return Result{}, f
	}
	ns := namespaceFor(fl, env)
	docs, f := readDocs(env, paths, ns, fl.str("namespace") != "")
	if f != nil {
		return Result{}, f
	}
	orphan := fl.str("cascade") == "orphan" || fl.str("cascade") == "false"
	var o outcome
	var objs []simulator.Object
	for _, d := range docs {
		objs = append(objs, d.obj)
	}
	noteParsed(p, ns, objs)
	for _, d := range docs {
		m := d.obj.Meta()
		t := typeOfKind(d.obj.GetKind())
		if err := env.State.Remove(d.obj.GetKind(), m.Namespace, m.Name, orphan); err != nil {
			var nf *simulator.NotFoundError
			if errors.As(err, &nf) && fl.boolean("ignore-not-found") {
				continue
			}
			if fail, internal := writeFailure(err); internal != nil {
				o.err = internal
			} else {
				o.failed(fileFailure(fail, "deleting", d.path))
			}
			continue
		}
		o.mutated = true
		if fl.str("output") == "name" {
			o.ok("%s/%s", t.qualified(), m.Name)
		} else {
			o.ok("%s %q deleted", t.qualified(), m.Name)
		}
	}
	return o.result()
}

// ---- apply -f ----

var applyFlags = append([]flagDef{
	{Long: "filename", Short: "f", Value: true},
	{Long: "prune"},
	{Long: "server-side"},
	{Long: "force"},
	{Long: "recursive", Short: "R"},
	{Long: "kustomize", Short: "k", Value: true},
	{Long: "selector", Short: "l", Value: true},
}, dryRunFlags...)

func runApply(args []string, env Env, p *Parsed) (Result, *Failure) {
	pos, fl, f := parseFlags(args, "apply", applyFlags)
	if f != nil {
		return Result{}, f
	}
	if r, ok := helpRequested(fl, "apply"); ok {
		return r, nil
	}
	recordFlags(p, fl)
	switch {
	case fl.boolean("server-side"):
		return Result{}, reject(CodeUnsupported, "error: server-side apply isn't simulated; kubectl apply without --server-side works the same for these lessons")
	case fl.boolean("prune") || fl.str("selector") != "":
		return Result{}, reject(CodeUnsupported, "error: --prune and -l aren't supported; delete objects you no longer want with kubectl delete")
	case fl.str("kustomize") != "":
		return Result{}, reject(CodeUnsupported, "error: kustomize (-k) isn't part of the simulator; apply the manifests with -f")
	case fl.boolean("force"):
		return Result{}, reject(CodeUnsupported, "error: apply --force isn't supported; delete the object and apply again")
	case len(pos) > 0 && len(fl.values["filename"]) == 0 && (pos[0] == "view-last-applied" || pos[0] == "set-last-applied" || pos[0] == "edit-last-applied"):
		return Result{}, reject(CodeUnsupported, "error: 'kubectl apply %s' isn't supported; see the annotation with kubectl get TYPE NAME -o yaml", pos[0])
	case len(pos) > 0:
		return Result{}, reject(CodeValidation, "error: Unexpected args: %v\nSee 'kubectl apply -h' for help and examples", pos)
	case len(fl.values["filename"]) == 0:
		return Result{}, reject(CodeValidation, "error: must specify one of -f and -k\nⓘ Save a manifest in the lab editor, then: kubectl apply -f manifests/<file>.yaml")
	}
	docs, dry, f := fileTargets(fl, env, p)
	if f != nil {
		return Result{}, f
	}
	var o outcome
	for _, d := range docs {
		verb, err := applyOne(env, d)
		if err != nil {
			action := "patching"
			if verb == "created" {
				action = "creating"
			}
			if fail, internal := writeFailure(err); internal != nil {
				o.err = internal
			} else {
				o.failed(fileFailure(fail, action, d.path))
			}
			continue
		}
		if verb == "" {
			continue
		}
		if verb != "unchanged" {
			o.mutated = true
		}
		if strings.HasPrefix(verb, "warning:") {
			o.stderr.WriteString(strings.TrimPrefix(verb, "warning:"))
			verb = "configured"
		}
		report(&o, d.obj, verb, dry, fl.str("output"))
	}
	if dry {
		o.mutated = false
	}
	return o.result()
}

// setLastApplied records the manifest as written in the object's annotation.
func setLastApplied(d doc) error {
	clean := cloneMap(d.raw)
	if meta, ok := clean["metadata"].(map[string]any); ok {
		if ann, ok := meta["annotations"].(map[string]any); ok {
			delete(ann, LastAppliedAnnotation)
			if len(ann) == 0 {
				delete(meta, "annotations")
			}
		}
	}
	delete(clean, "status")
	b, err := json.Marshal(clean)
	if err != nil {
		return err
	}
	m := d.obj.Meta()
	if m.Annotations == nil {
		m.Annotations = map[string]string{}
	}
	m.Annotations[LastAppliedAnnotation] = string(b) + "\n"
	return nil
}

// applyOne creates the object, or three-way merges the manifest into the live object. It
// returns kubectl's verb: created, configured or unchanged; a missing last-applied annotation
// is reported with kubectl's warning (prefixed "warning:").
func applyOne(env Env, d doc) (string, error) {
	if err := setLastApplied(d); err != nil {
		return "", err
	}
	m := d.obj.Meta()
	live := env.State.Get(d.obj.GetKind(), m.Namespace, m.Name)
	if live != nil && live.GetKind() == simulator.KindNode {
		return "", &simulator.ForbiddenError{Kind: simulator.KindNode, Name: m.Name, Reason: "nodes are part of the simulated cluster; change their labels with kubectl label node"}
	}
	if live == nil {
		if err := env.State.Create(d.obj); err != nil {
			return "created", err
		}
		return "created", nil
	}
	liveGen, err := generic(live)
	if err != nil {
		return "", err
	}
	warning := ""
	var last map[string]any
	if s := live.Meta().Annotations[LastAppliedAnnotation]; s != "" {
		_ = json.Unmarshal([]byte(s), &last)
	} else {
		t := typeOfKind(live.GetKind())
		warning = fmt.Sprintf("Warning: resource %s/%s is missing the %s annotation which is required by kubectl apply. kubectl apply should only be used on resources created declaratively by either kubectl create --save-config or kubectl apply. The missing annotation will be patched automatically.\n", t.display(), m.Name, LastAppliedAnnotation)
	}
	desired, err := generic(d.obj)
	if err != nil {
		return "", err
	}
	// Merge only what the manifest says (its generic form also has empty metadata fields).
	want := cloneMap(d.raw)
	wantMeta, _ := want["metadata"].(map[string]any)
	if ann, ok := desired["metadata"].(map[string]any)["annotations"]; ok && wantMeta != nil {
		wantMeta["annotations"] = ann
	}
	delete(want, "status")
	merged := merge3(liveGen, last, want)
	raw, err := json.Marshal(merged)
	if err != nil {
		return "", err
	}
	obj, err := simulator.DecodeInput(raw)
	if err != nil {
		return "", err
	}
	before := env.State.Revision
	if err := env.State.Update(obj); err != nil {
		return "", err
	}
	if env.State.Revision == before {
		return "unchanged", nil
	}
	if warning != "" {
		return "warning:" + warning, nil
	}
	return "configured", nil
}

func generic(o simulator.Object) (map[string]any, error) {
	b, err := json.Marshal(o)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	dec := json.NewDecoder(bytes.NewReader(b))
	return m, dec.Decode(&m)
}

func cloneMap(m map[string]any) map[string]any {
	b, _ := json.Marshal(m)
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}

// merge3 is kubectl's client-side three-way merge on the supported field subset: fields in the
// manifest replace live values, fields the previous apply set but the manifest no longer has
// are removed, and anything else in the live object (set by controllers or other commands) is
// kept. Lists of containers, env vars and ports merge by their key, as a strategic merge patch
// does; other lists are replaced.
func merge3(live, last, desired map[string]any) map[string]any {
	out := make(map[string]any, len(live))
	for k, v := range live {
		out[k] = v
	}
	for k, dv := range desired {
		if dv == nil {
			delete(out, k)
			continue
		}
		lastV := last[k]
		switch d := dv.(type) {
		case map[string]any:
			lm, ok := out[k].(map[string]any)
			lastM, _ := lastV.(map[string]any)
			if ok {
				out[k] = merge3(lm, lastM, d)
			} else {
				out[k] = d
			}
		case []any:
			ll, ok := out[k].([]any)
			key := mergeKey(k, d, ll)
			if ok && key != "" {
				lastL, _ := lastV.([]any)
				out[k] = mergeList(ll, lastL, d, key)
			} else {
				out[k] = d
			}
		default:
			out[k] = d
		}
	}
	for k := range last {
		if _, still := desired[k]; !still {
			delete(out, k)
		}
	}
	return out
}

// mergeKey returns the strategic-merge key for a list field, or "" to replace the list.
func mergeKey(field string, desired, live []any) string {
	switch field {
	case "containers", "env":
		return "name"
	case "ports":
		for _, items := range [][]any{desired, live} {
			for _, it := range items {
				if m, ok := it.(map[string]any); ok {
					if _, ok := m["containerPort"]; ok {
						return "containerPort"
					}
					if _, ok := m["port"]; ok {
						return "port"
					}
				}
			}
		}
	}
	return ""
}

func mergeList(live, last, desired []any, key string) []any {
	index := func(items []any) map[string]map[string]any {
		out := map[string]map[string]any{}
		for _, it := range items {
			if m, ok := it.(map[string]any); ok && m[key] != nil {
				out[fmt.Sprint(m[key])] = m
			}
		}
		return out
	}
	liveIdx, lastIdx := index(live), index(last)
	var out []any
	seen := map[string]bool{}
	for _, it := range desired {
		m, ok := it.(map[string]any)
		if !ok || m[key] == nil {
			return desired // malformed for merging: replace, and let validation report it
		}
		k := fmt.Sprint(m[key])
		seen[k] = true
		if lm, ok := liveIdx[k]; ok {
			out = append(out, merge3(lm, lastIdx[k], m))
		} else {
			out = append(out, m)
		}
	}
	for _, it := range live {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		k := fmt.Sprint(m[key])
		if seen[k] {
			continue
		}
		if _, applied := lastIdx[k]; applied {
			continue // the manifest used to set it and no longer does
		}
		out = append(out, m)
	}
	return out
}
