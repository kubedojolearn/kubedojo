package kubectl

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// ---- delete ----

var deleteFlags = []flagDef{
	{Long: "selector", Short: "l", Value: true},
	{Long: "all"},
	{Long: "cascade", Value: true},
	{Long: "force"},
	{Long: "grace-period", Value: true},
	{Long: "now"},
	{Long: "wait"},
	{Long: "ignore-not-found"},
	{Long: "filename", Short: "f", Value: true},
	{Long: "output", Short: "o", Value: true},
}

func runDelete(args []string, env Env, p *Parsed) (Result, *Failure) {
	pos, fl, f := parseFlags(args, "delete", deleteFlags)
	if f != nil {
		return Result{}, f
	}
	if r, ok := helpRequested(fl, "delete"); ok {
		return r, nil
	}
	recordFlags(p, fl)
	if fl.str("filename") != "" {
		return deleteFromFile(fl, env, p, pos)
	}
	output := fl.str("output")
	if f := checkOutput(output, "name"); f != nil {
		return Result{}, f
	}
	orphan := false
	switch c := fl.str("cascade"); c {
	case "", "background", "foreground", "true":
	case "orphan", "false":
		orphan = true
	default:
		return Result{}, reject(CodeValidation, "error: invalid cascade value (%s). Must be \"background\", \"foreground\", or \"orphan\"", c)
	}
	if s := fl.str("grace-period"); s != "" {
		if _, err := strconv.Atoi(s); err != nil {
			return Result{}, reject(CodeValidation, "error: invalid argument %q for \"--grace-period\" flag", s)
		}
	}
	var o outcome
	if fl.boolean("force") {
		o.stderr.WriteString("Warning: Immediate deletion does not wait for confirmation that the running resource has been terminated. The resource may continue to run on the cluster indefinitely.\n")
	}
	ns := namespaceFor(fl, env)
	objs, f := writableTargets(env, pos, fl, "delete", &o)
	if f != nil {
		return Result{Stderr: o.stderr.String()}, f
	}
	if fl.boolean("ignore-not-found") && o.failure != nil && o.failure.Code == CodeNotFound {
		var kept strings.Builder
		for _, line := range strings.SplitAfter(o.stderr.String(), "\n") {
			if !strings.HasPrefix(line, "Error from server (NotFound)") {
				kept.WriteString(line)
			}
		}
		o.stderr.Reset()
		o.stderr.WriteString(kept.String())
		o.failure = nil
	}
	noteParsed(p, ns, objs)
	if len(objs) == 0 && o.failure == nil && o.stderr.Len() == 0 && !namedTargets(pos) {
		o.stderr.WriteString("No resources found\n")
	}
	for _, obj := range objs {
		m := obj.Meta()
		t := typeOfKind(obj.GetKind())
		if !env.State.Has(obj.GetKind(), m.Namespace, m.Name) {
			continue // already removed with its namespace earlier in this command
		}
		if err := env.State.Remove(obj.GetKind(), m.Namespace, m.Name, orphan); err != nil {
			o.fail(err)
			continue
		}
		o.mutated = true
		if output == "name" {
			o.ok("%s/%s", t.qualified(), m.Name)
		} else {
			o.ok("%s %q deleted", t.qualified(), m.Name)
		}
	}
	return o.result()
}

// namedTargets reports whether TYPE NAME or TYPE/NAME arguments name specific objects.
func namedTargets(pos []string) bool {
	return len(pos) > 1 || len(pos) == 1 && strings.Contains(pos[0], "/")
}

// ---- scale ----

func runScale(args []string, env Env, p *Parsed) (Result, *Failure) {
	pos, fl, f := parseFlags(args, "scale", []flagDef{
		{Long: "replicas", Value: true}, {Long: "current-replicas", Value: true},
		{Long: "selector", Short: "l", Value: true}, {Long: "all"}, {Long: "filename", Short: "f", Value: true},
		{Long: "timeout", Value: true}, {Long: "output", Short: "o", Value: true},
	})
	if f != nil {
		return Result{}, f
	}
	if r, ok := helpRequested(fl, "scale"); ok {
		return r, nil
	}
	recordFlags(p, fl)
	if fl.str("filename") != "" {
		return Result{}, reject(CodeUnsupported, "error: scale -f isn't supported; use kubectl scale deployment NAME --replicas=N")
	}
	output := fl.str("output")
	if f := checkOutput(output, "name"); f != nil {
		return Result{}, f
	}
	s := fl.str("replicas")
	if s == "" {
		return Result{}, reject(CodeValidation, "error: required flag(s) \"replicas\" not set")
	}
	replicas, err := strconv.ParseInt(s, 10, 32)
	if err != nil || replicas < 0 {
		return Result{}, reject(CodeValidation, "error: The --replicas=COUNT flag is required, and COUNT must be greater than or equal to 0")
	}
	current := int64(-1)
	if c := fl.str("current-replicas"); c != "" {
		if current, err = strconv.ParseInt(c, 10, 32); err != nil || current < -1 {
			return Result{}, reject(CodeValidation, "error: invalid argument %q for \"--current-replicas\" flag", c)
		}
	}
	var o outcome
	ns := namespaceFor(fl, env)
	objs, f := writableTargets(env, pos, fl, "scale", &o)
	if f != nil {
		return Result{}, f
	}
	noteParsed(p, ns, objs)
	if len(objs) == 0 && o.failure == nil {
		o.stderr.WriteString("No resources found\n")
	}
	n := int32(replicas)
	for _, obj := range objs {
		t := typeOfKind(obj.GetKind())
		var have *int32
		switch v := obj.(type) {
		case *simulator.Deployment:
			have = v.Spec.Replicas
			v.Spec.Replicas = &n
		case *simulator.ReplicaSet:
			have = v.Spec.Replicas
			v.Spec.Replicas = &n
		default:
			o.failed(reject(CodeUnsupported, "error: no scaler has been implemented for %s: the simulator scales deployments and replicasets", t.display()))
			continue
		}
		if current >= 0 && (have == nil || int64(*have) != current) {
			was := int32(1)
			if have != nil {
				was = *have
			}
			o.failed(reject(CodeValidation, "error: Expected replicas to be %d, was %d", current, was))
			continue
		}
		if err := env.State.Update(obj); err != nil {
			o.fail(err)
			continue
		}
		o.mutated = true
		if output == "name" {
			o.ok("%s/%s", t.qualified(), obj.Meta().Name)
		} else {
			o.ok("%s/%s scaled", t.qualified(), obj.Meta().Name)
		}
	}
	return o.result()
}

// ---- set image / set env ----

func runSet(args []string, env Env, p *Parsed) (Result, *Failure) {
	pos, fl, f := parseFlags(args, "set", []flagDef{
		{Long: "selector", Short: "l", Value: true}, {Long: "all"}, {Long: "container", Short: "c", Value: true},
		{Long: "from", Value: true}, {Long: "prefix", Value: true}, {Long: "keys", Value: true},
		{Long: "filename", Short: "f", Value: true}, {Long: "local"}, {Long: "list"},
		{Long: "dry-run", Value: true}, {Long: "output", Short: "o", Value: true},
		{Long: "limits", Value: true}, {Long: "requests", Value: true},
	})
	if f != nil {
		return Result{}, f
	}
	if r, ok := helpRequested(fl, "set"); ok {
		return r, nil
	}
	recordFlags(p, fl)
	if len(pos) == 0 {
		return Result{Stdout: commandHelp["set"]}, nil
	}
	sub := pos[0]
	p.Subcommand = sub
	switch sub {
	case "image", "env":
	case "resources":
		return Result{}, reject(CodeUnsupported, "error: 'kubectl set resources' isn't supported yet; change resources in the manifest and use kubectl apply -f")
	default:
		return Result{}, reject(CodeUnsupported, "error: 'kubectl set %s' isn't supported in the simulator; it supports set image and set env", truncateWord(sub))
	}
	switch {
	case fl.str("filename") != "" || fl.boolean("local"):
		return Result{}, reject(CodeUnsupported, "error: set %s -f/--local isn't supported; name the resource, e.g. kubectl set %s deployment/web …", sub, sub)
	case fl.boolean("list"):
		return Result{}, reject(CodeUnsupported, "error: --list isn't supported; see a Deployment's environment with kubectl describe deployment NAME")
	case fl.str("dry-run") != "" || fl.str("output") != "":
		return Result{}, reject(CodeUnsupported, "error: --dry-run and -o aren't supported for kubectl set")
	case fl.str("keys") != "" || fl.str("prefix") != "":
		return Result{}, reject(CodeUnsupported, "error: --keys and --prefix aren't supported; --from imports every key")
	}
	// Arguments with '=' (or ending in '-' for set env) are changes; the rest name resources.
	var resources, changes []string
	for _, a := range pos[1:] {
		if strings.Contains(a, "=") || (sub == "env" && len(a) > 1 && strings.HasSuffix(a, "-")) {
			changes = append(changes, a)
		} else {
			resources = append(resources, a)
		}
	}
	if sub == "image" && len(changes) == 0 {
		return Result{}, reject(CodeValidation, "error: at least one image update is required, e.g. kubectl set image deployment/web nginx=nginx:1.27")
	}
	if sub == "env" && len(changes) == 0 && fl.str("from") == "" {
		return Result{}, reject(CodeValidation, "error: at least one environment variable must be provided, e.g. kubectl set env deployment/web MODE=prod")
	}
	var o outcome
	ns := namespaceFor(fl, env)
	objs, f := writableTargets(env, resources, fl, "set "+sub, &o)
	if f != nil {
		return Result{}, f
	}
	noteParsed(p, ns, objs)
	for _, obj := range objs {
		t := typeOfKind(obj.GetKind())
		var tmpl *simulator.PodSpec
		switch v := obj.(type) {
		case *simulator.Deployment:
			tmpl = &v.Spec.Template.Spec
		case *simulator.ReplicaSet:
			tmpl = &v.Spec.Template.Spec
		case *simulator.Pod:
			o.failed(reject(CodeUnsupported, "error: changing a Pod's containers in place isn't simulated; change its Deployment instead (Pods are replaced, not edited)"))
			continue
		default:
			o.failed(reject(CodeValidation, "error: %s/%s has no containers to change", t.qualified(), obj.Meta().Name))
			continue
		}
		var f *Failure
		if sub == "image" {
			f = setImages(tmpl, changes)
		} else {
			f = setEnv(env, ns, tmpl, changes, fl.str("from"), fl.str("container"))
		}
		if f != nil {
			o.failed(f)
			continue
		}
		if err := env.State.Update(obj); err != nil {
			o.fail(err)
			continue
		}
		o.mutated = true
		o.ok("%s/%s %s updated", t.qualified(), obj.Meta().Name, sub)
	}
	return o.result()
}

func setImages(spec *simulator.PodSpec, changes []string) *Failure {
	for _, ch := range changes {
		name, image, _ := strings.Cut(ch, "=")
		if image == "" {
			return reject(CodeValidation, "error: invalid image update %q, expected CONTAINER=IMAGE", ch)
		}
		found := false
		for i := range spec.Containers {
			if name == "*" || spec.Containers[i].Name == name {
				spec.Containers[i].Image = image
				found = true
			}
		}
		if !found {
			return reject(CodeNotFound, "error: unable to find container named %q", name)
		}
	}
	return nil
}

func setEnv(env Env, ns string, spec *simulator.PodSpec, changes []string, from, container string) *Failure {
	var sets []simulator.EnvVar
	var removes []string
	for _, ch := range changes {
		if name, ok := strings.CutSuffix(ch, "-"); ok && !strings.Contains(ch, "=") {
			removes = append(removes, name)
			continue
		}
		k, v, _ := strings.Cut(ch, "=")
		if k == "" {
			return reject(CodeValidation, "error: invalid environment variable %q, expected KEY=VALUE or KEY-", ch)
		}
		sets = append(sets, simulator.EnvVar{Name: k, Value: v})
	}
	if from != "" {
		typ, name, ok := strings.Cut(from, "/")
		if !ok || name == "" {
			return reject(CodeValidation, "error: --from must be configmap/NAME or secret/NAME")
		}
		t, f := lookupType(typ)
		if f != nil {
			return f
		}
		switch t.Kind {
		case simulator.KindConfigMap:
			cm, ok := env.State.Get(simulator.KindConfigMap, ns, name).(*simulator.ConfigMap)
			if !ok {
				return reject(CodeNotFound, "error: configmaps %q not found", name)
			}
			for _, k := range sortedKeys(cm.Data) {
				sets = append(sets, simulator.EnvVar{Name: k, ValueFrom: &simulator.EnvVarSource{ConfigMapKeyRef: &simulator.KeySelector{Name: name, Key: k}}})
			}
		case simulator.KindSecret:
			s, ok := env.State.Get(simulator.KindSecret, ns, name).(*simulator.Secret)
			if !ok {
				return reject(CodeNotFound, "error: secrets %q not found", name)
			}
			for _, k := range sortedKeys(s.Data) {
				sets = append(sets, simulator.EnvVar{Name: k, ValueFrom: &simulator.EnvVarSource{SecretKeyRef: &simulator.KeySelector{Name: name, Key: k}}})
			}
		default:
			return reject(CodeValidation, "error: --from must be configmap/NAME or secret/NAME")
		}
	}
	found := false
	for i := range spec.Containers {
		c := &spec.Containers[i]
		if container != "" && c.Name != container {
			continue
		}
		found = true
		for _, name := range removes {
			c.Env = slices.DeleteFunc(c.Env, func(e simulator.EnvVar) bool { return e.Name == name })
		}
		for _, e := range sets {
			if i := slices.IndexFunc(c.Env, func(x simulator.EnvVar) bool { return x.Name == e.Name }); i >= 0 {
				c.Env[i] = e
			} else {
				c.Env = append(c.Env, e)
			}
		}
		if len(c.Env) == 0 {
			c.Env = nil
		}
	}
	if !found {
		return reject(CodeNotFound, "error: unable to find container named %q", container)
	}
	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// ---- label / annotate ----

func runLabel(args []string, env Env, p *Parsed) (Result, *Failure) {
	return runMetaChange(args, env, p, "label")
}

func runAnnotate(args []string, env Env, p *Parsed) (Result, *Failure) {
	return runMetaChange(args, env, p, "annotate")
}

// runMetaChange implements label and annotate: KEY=VALUE sets, KEY- removes.
func runMetaChange(args []string, env Env, p *Parsed, verb string) (Result, *Failure) {
	pos, fl, f := parseFlags(args, verb, []flagDef{
		{Long: "overwrite"}, {Long: "selector", Short: "l", Value: true}, {Long: "all"},
		{Long: "list"}, {Long: "local"}, {Long: "filename", Short: "f", Value: true},
		{Long: "resource-version", Value: true}, {Long: "dry-run", Value: true}, {Long: "output", Short: "o", Value: true},
	})
	if f != nil {
		return Result{}, f
	}
	if r, ok := helpRequested(fl, verb); ok {
		return r, nil
	}
	recordFlags(p, fl)
	switch {
	case fl.boolean("list"):
		return Result{}, reject(CodeUnsupported, "error: --list isn't supported; use kubectl get TYPE NAME --show-labels or kubectl describe")
	case fl.boolean("local") || fl.str("filename") != "":
		return Result{}, reject(CodeUnsupported, "error: %s -f/--local isn't supported; name the resource, e.g. kubectl %s pod web app=web", verb, verb)
	case fl.str("resource-version") != "" || fl.str("dry-run") != "" || fl.str("output") != "":
		return Result{}, reject(CodeUnsupported, "error: --resource-version, --dry-run and -o aren't supported for kubectl %s", verb)
	}
	noun := map[string]string{"label": "label", "annotate": "annotation"}[verb]
	var resources []string
	sets := map[string]string{}
	var setOrder, removes []string
	for _, a := range pos {
		switch {
		case strings.Contains(a, "="):
			k, v, _ := strings.Cut(a, "=")
			if err := simulator.ValidateLabelKey(k); err != nil {
				return Result{}, reject(CodeValidation, "error: invalid %s spec: %s: %s", noun, a, err.Error())
			}
			if verb == "label" {
				if reasons := simulator.ValidateLabels(map[string]string{k: v}); len(reasons) > 0 {
					return Result{}, reject(CodeValidation, "error: invalid label spec: %s: %s", a, strings.Join(reasons, "; "))
				}
			}
			if _, dup := sets[k]; !dup {
				setOrder = append(setOrder, k)
			}
			sets[k] = v
		case len(a) > 1 && strings.HasSuffix(a, "-") && len(resources) > 0:
			removes = append(removes, strings.TrimSuffix(a, "-"))
		default:
			resources = append(resources, a)
		}
	}
	if len(sets) == 0 && len(removes) == 0 {
		return Result{}, reject(CodeValidation, "error: at least one %s update is required, e.g. kubectl %s pod web app=web", noun, verb)
	}
	var o outcome
	ns := namespaceFor(fl, env)
	objs, f := writableTargets(env, resources, fl, verb, &o)
	if f != nil {
		return Result{}, f
	}
	noteParsed(p, ns, objs)
	if len(objs) == 0 && o.failure == nil {
		o.stderr.WriteString("No resources found\n")
	}
	done := map[string]string{"label": "labeled", "annotate": "annotated"}[verb]
	for _, obj := range objs {
		t := typeOfKind(obj.GetKind())
		m := obj.Meta()
		target := &m.Labels
		if verb == "annotate" {
			target = &m.Annotations
		}
		changed, f := changeMap(target, sets, setOrder, removes, fl.boolean("overwrite"), noun, &o)
		if f != nil {
			o.failed(f)
			continue
		}
		if !changed {
			o.ok("%s/%s not %s", t.qualified(), m.Name, done)
			continue
		}
		if err := env.State.Update(obj); err != nil {
			o.fail(err)
			continue
		}
		o.mutated = true
		o.ok("%s/%s %s", t.qualified(), m.Name, done)
	}
	return o.result()
}

func changeMap(target *map[string]string, sets map[string]string, order, removes []string, overwrite bool, noun string, o *outcome) (bool, *Failure) {
	m := *target
	for _, k := range order {
		if old, ok := m[k]; ok && old != sets[k] && !overwrite {
			return false, reject(CodeValidation, "error: '%s' already has a value (%s), and --overwrite is false", k, old)
		}
	}
	changed := false
	for _, k := range order {
		if m == nil {
			m = map[string]string{}
		}
		if old, ok := m[k]; !ok || old != sets[k] {
			m[k] = sets[k]
			changed = true
		}
	}
	for _, k := range removes {
		if _, ok := m[k]; !ok {
			fmt.Fprintf(&o.stderr, "%s %q not found.\n", noun, k)
			continue
		}
		delete(m, k)
		changed = true
	}
	if len(m) == 0 {
		m = nil
	}
	*target = m
	return changed, nil
}

// ---- rollout undo / restart ----

// RestartedAtAnnotation is set on the pod template by `kubectl rollout restart`.
const RestartedAtAnnotation = "kubectl.kubernetes.io/restartedAt"

func rolloutUndo(env Env, d *simulator.Deployment, toRevision string) (Result, *Failure) {
	want := int64(0)
	if toRevision != "" {
		n, err := strconv.ParseInt(toRevision, 10, 64)
		if err != nil || n < 0 {
			return Result{}, reject(CodeValidation, "error: invalid argument %q for \"--to-revision\" flag", toRevision)
		}
		want = n
	}
	type rev struct {
		n  int64
		rs *simulator.ReplicaSet
	}
	var revs []rev
	for _, o := range env.State.List(simulator.KindReplicaSet, d.Metadata.Namespace) {
		rs := o.(*simulator.ReplicaSet)
		if simulator.ControllerOf(rs) != d.Metadata.UID {
			continue
		}
		if n, err := strconv.ParseInt(rs.Metadata.Annotations[simulator.RevisionAnnotation], 10, 64); err == nil {
			revs = append(revs, rev{n, rs})
		}
	}
	slices.SortFunc(revs, func(a, b rev) int { return int(a.n - b.n) })
	name := "deployment.apps/" + d.Metadata.Name
	var target *rev
	switch {
	case want == 0 && len(revs) < 2:
		return Result{}, reject(CodeNotFound, "error: no rollout history found for deployment %q", d.Metadata.Name)
	case want == 0:
		target = &revs[len(revs)-2]
	default:
		for i := range revs {
			if revs[i].n == want {
				target = &revs[i]
			}
		}
		if target == nil {
			return Result{}, reject(CodeNotFound, "error: unable to find specified revision %d in history", want)
		}
	}
	tmpl := target.rs.Spec.Template
	tmpl.Metadata.Labels = copyMap(tmpl.Metadata.Labels)
	delete(tmpl.Metadata.Labels, simulator.PodTemplateHashLabel)
	if simulator.TemplateHash(tmpl) == simulator.TemplateHash(d.Spec.Template) {
		return Result{Stdout: fmt.Sprintf("%s skipped rollback (current template already matches revision %d)\n", name, target.n)}, nil
	}
	d.Spec.Template = tmpl
	if err := env.State.Update(d); err != nil {
		f, internal := writeFailure(err)
		return Result{Err: internal}, f
	}
	return Result{Stdout: name + " rolled back\n", Mutated: true}, nil
}

func rolloutRestart(env Env, d *simulator.Deployment) (Result, *Failure) {
	if d.Spec.Template.Metadata.Annotations == nil {
		d.Spec.Template.Metadata.Annotations = map[string]string{}
	}
	d.Spec.Template.Metadata.Annotations[RestartedAtAnnotation] = env.State.Now.UTC().Format(time.RFC3339)
	if err := env.State.Update(d); err != nil {
		f, internal := writeFailure(err)
		return Result{Err: internal}, f
	}
	return Result{Stdout: "deployment.apps/" + d.Metadata.Name + " restarted\n", Mutated: true}, nil
}

// ---- config set-context ----

func setContext(pos []string, fl flags, env Env) (Result, *Failure) {
	if len(pos) > 2 || (len(pos) == 2 && pos[1] != contextName) {
		return Result{}, reject(CodeUnsupported, "error: the simulator has a single context (%s); use kubectl config set-context --current --namespace=NAME", contextName)
	}
	if !fl.boolean("current") && len(pos) < 2 {
		return Result{}, reject(CodeValidation, "error: you must specify a context name or --current")
	}
	ns := fl.str("namespace")
	if ns == "" {
		return Result{}, reject(CodeUnsupported, "error: only --namespace can be changed in the simulator: kubectl config set-context --current --namespace=NAME")
	}
	if err := simulator.ValidateName(simulator.KindNamespace, ns); err != nil {
		return Result{}, reject(CodeValidation, "error: %q is not a valid namespace name", ns)
	}
	env.State.CurrentNamespace = ns
	res := Result{Stdout: fmt.Sprintf("Context %q modified.\n", contextName), Mutated: true}
	if !namespaceExists(env, ns) {
		res.Stderr = fmt.Sprintf("ⓘ The namespace %q doesn't exist yet. Create it with: kubectl create namespace %s\n", ns, ns)
	}
	return res, nil
}

// ---- edit / patch / replace / apply ----

func runEdit(_ []string, _ Env, _ *Parsed) (Result, *Failure) {
	return Result{}, reject(CodeUnsupported, "error: 'kubectl edit' opens a text editor, which the browser terminal doesn't have. Change the manifest in the lab's YAML editor, then run kubectl apply -f <file>.")
}

func runPatch(_ []string, _ Env, _ *Parsed) (Result, *Failure) {
	return Result{}, reject(CodeUnsupported, "error: 'kubectl patch' isn't supported in the simulator. Use kubectl apply -f, kubectl set, kubectl scale or kubectl label instead.")
}

func runReplace(_ []string, _ Env, _ *Parsed) (Result, *Failure) {
	return Result{}, reject(CodeUnsupported, "error: 'kubectl replace' isn't supported in the simulator. Use kubectl apply -f <file>.")
}
