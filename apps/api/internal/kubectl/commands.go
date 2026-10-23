package kubectl

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// The simulated control plane address (simulator-compatibility.md §3).
const (
	serverURL     = "https://kubedojo.sim:6443"
	clientVersion = "v1.37.1"
	contextName   = "kubedojo"
)

func helpRequested(fl flags, cmd string) (Result, bool) {
	if fl.boolean("help") {
		return Result{Stdout: commandHelp[cmd]}, true
	}
	return Result{}, false
}

// ---- logs ----

var logsFlags = []flagDef{
	{Long: "container", Short: "c", Value: true},
	{Long: "previous", Short: "p"},
	{Long: "tail", Value: true},
	{Long: "follow", Short: "f"},
	{Long: "timestamps"},
	{Long: "all-containers"},
}

func runLogs(args []string, env Env, p *Parsed) (Result, *Failure) {
	pos, fl, f := parseFlags(args, "logs", logsFlags)
	if f != nil {
		return Result{}, f
	}
	if r, ok := helpRequested(fl, "logs"); ok {
		return r, nil
	}
	recordFlags(p, fl)
	if fl.boolean("follow") {
		return Result{}, reject(CodeUnsupported, "error: -f/--follow isn't supported in the simulator; run the command again to see new output")
	}
	if fl.boolean("timestamps") || fl.boolean("all-containers") {
		return Result{}, reject(CodeUnsupported, "error: --timestamps and --all-containers aren't supported in the simulator")
	}
	tail := -1
	if s := fl.str("tail"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < -1 {
			return Result{}, reject(CodeValidation, "error: invalid argument %q for \"--tail\" flag: must be an integer", s)
		}
		tail = n
	}
	if len(pos) == 0 {
		return Result{}, reject(CodeValidation, "error: expected 'logs [-p] (POD | TYPE/NAME) [-c CONTAINER]'.\nPOD or TYPE/NAME is a required argument for the logs command\nSee 'kubectl logs -h' for help and examples")
	}
	if len(pos) > 2 {
		return Result{}, reject(CodeValidation, "error: expected 'logs [-p] (POD | TYPE/NAME) [-c CONTAINER]'.")
	}
	ns := namespaceFor(fl, env)
	p.Namespace, p.Resources = ns, []string{simulator.KindPod}
	var stderr strings.Builder

	pod, f := logsPod(env, ns, pos[0], &stderr)
	if f != nil {
		return Result{Stderr: stderr.String()}, f
	}
	p.Names = []string{pod.Metadata.Name}
	container := fl.str("container")
	if len(pos) == 2 {
		container = pos[1]
	}
	if container == "" {
		container = pod.Spec.Containers[0].Name
		if len(pod.Spec.Containers) > 1 {
			var names []string
			for _, c := range pod.Spec.Containers {
				names = append(names, c.Name)
			}
			fmt.Fprintf(&stderr, "Defaulted container %q out of: %s\n", container, strings.Join(names, ", "))
		}
	}
	var spec *simulator.Container
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == container {
			spec = &pod.Spec.Containers[i]
		}
	}
	if spec == nil {
		return Result{Stderr: stderr.String()}, reject(CodeNotFound, "error: container %s is not valid for pod %s", container, pod.Metadata.Name)
	}
	var cs *simulator.ContainerStatus
	for i := range pod.Status.ContainerStatuses {
		if pod.Status.ContainerStatuses[i].Name == container {
			cs = &pod.Status.ContainerStatuses[i]
		}
	}
	badRequest := func(format string, a ...any) *Failure {
		return reject(CodeValidation, "Error from server (BadRequest): "+format, a...)
	}
	if fl.boolean("previous") {
		if cs == nil || cs.LastState.Terminated == nil {
			return Result{Stderr: stderr.String()}, badRequest("previous terminated container %q in pod %q not found", container, pod.Metadata.Name)
		}
	} else if cs == nil || cs.State.Waiting != nil && cs.State.Waiting.Reason != "CrashLoopBackOff" {
		reason := "ContainerCreating"
		if cs != nil && cs.State.Waiting != nil {
			reason = cs.State.Waiting.Reason
			if reason == "ImagePullBackOff" || reason == "ErrImagePull" {
				reason = "trying and failing to pull image"
			}
		}
		return Result{Stderr: stderr.String()}, badRequest("container %q in pod %q is waiting to start: %s", container, pod.Metadata.Name, reason)
	}
	profile, _ := env.State.Images.Lookup(spec.Image)
	lines := profile.Logs
	if tail >= 0 && tail < len(lines) {
		lines = lines[len(lines)-tail:]
	}
	out := ""
	if len(lines) > 0 {
		out = strings.Join(lines, "\n") + "\n"
	}
	return Result{Stdout: out, Stderr: stderr.String()}, nil
}

// logsPod resolves POD, pod/NAME, deployment/NAME or replicaset/NAME to a Pod.
func logsPod(env Env, ns, arg string, stderr *strings.Builder) (*simulator.Pod, *Failure) {
	typ, name, hasType := strings.Cut(arg, "/")
	if !hasType {
		typ, name = "pod", arg
	}
	t, f := lookupType(typ)
	if f != nil {
		return nil, f
	}
	switch t.Kind {
	case simulator.KindPod:
		if o, ok := env.State.Get(simulator.KindPod, ns, name).(*simulator.Pod); ok {
			return o, nil
		}
		return nil, reject(CodeNotFound, "%s%s", strings.TrimSuffix(notFound(t, name), "\n"), strings.TrimSuffix("\n"+namespaceHint(env, ns), "\n"))
	case simulator.KindDeployment, simulator.KindReplicaSet:
		owner := env.State.Get(t.Kind, ns, name)
		if owner == nil {
			return nil, reject(CodeNotFound, "%s", strings.TrimSuffix(notFound(t, name), "\n"))
		}
		var pods []*simulator.Pod
		for _, o := range env.State.List(simulator.KindPod, ns) {
			pod := o.(*simulator.Pod)
			if ownedByChain(env.State, pod, owner.Meta().UID) {
				pods = append(pods, pod)
			}
		}
		if len(pods) == 0 {
			return nil, reject(CodeNotFound, "error: timed out waiting for the condition")
		}
		if len(pods) > 1 {
			fmt.Fprintf(stderr, "Found %d pods, using pod/%s\n", len(pods), pods[0].Metadata.Name)
		}
		return pods[0], nil
	}
	return nil, reject(CodeUnsupported, "error: cannot get the logs from %s: only pods, deployments and replicasets have logs", t.display())
}

// ownedByChain reports whether the pod is controlled by uid directly or through a ReplicaSet.
func ownedByChain(st *simulator.State, pod *simulator.Pod, uid string) bool {
	owner := simulator.ControllerOf(pod)
	if owner == uid {
		return true
	}
	for _, o := range st.List(simulator.KindReplicaSet, pod.Metadata.Namespace) {
		if o.Meta().UID == owner && simulator.ControllerOf(o) == uid {
			return true
		}
	}
	return false
}

// ---- events ----

var eventsFlags = []flagDef{
	{Long: "for", Value: true},
	{Long: "types", Value: true},
	{Long: "output", Short: "o", Value: true},
	{Long: "no-headers"},
}

func runEvents(args []string, env Env, p *Parsed) (Result, *Failure) {
	pos, fl, f := parseFlags(args, "events", eventsFlags)
	if f != nil {
		return Result{}, f
	}
	if r, ok := helpRequested(fl, "events"); ok {
		return r, nil
	}
	recordFlags(p, fl)
	if len(pos) > 0 {
		return Result{}, reject(CodeValidation, "error: unknown argument %q; use --for TYPE/NAME to filter events", pos[0])
	}
	output := fl.str("output")
	if f := checkOutput(output, "json", "yaml"); f != nil {
		return Result{}, f
	}
	ns, all := namespaceFor(fl, env), fl.boolean("all-namespaces")
	p.Namespace, p.AllNamespaces, p.Output, p.Resources = ns, all, output, []string{kindEvent}

	var forKind, forName string
	if s := fl.str("for"); s != "" {
		typ, name, ok := strings.Cut(s, "/")
		if !ok || name == "" {
			return Result{}, reject(CodeValidation, "error: --for must be in resource/name form, e.g. --for pod/web-1")
		}
		t, f := lookupType(typ)
		if f != nil {
			return Result{}, f
		}
		forKind, forName = t.Kind, name
	}
	types := map[string]bool{}
	if s := fl.str("types"); s != "" {
		for _, t := range strings.Split(s, ",") {
			switch strings.ToLower(strings.TrimSpace(t)) {
			case "normal":
				types["Normal"] = true
			case "warning":
				types["Warning"] = true
			default:
				return Result{}, reject(CodeValidation, "error: valid --types are Normal or Warning, got %q", t)
			}
		}
	}
	var items []item
	for _, it := range listItems(env, typeOfKind(kindEvent), ns, all) {
		if forKind != "" && (it.event.Kind != forKind || it.event.Name != forName) {
			continue
		}
		if len(types) > 0 && !types[it.event.Type] {
			continue
		}
		items = append(items, it)
	}
	if output != "" {
		out, red, err := printStructured(items, false, output)
		if err != nil {
			return Result{}, reject(CodeValidation, "error: %s", err.Error())
		}
		return Result{Stdout: out, Redacted: red}, nil
	}
	if len(items) == 0 {
		if all {
			return Result{Stderr: "No events found.\n"}, nil
		}
		return Result{Stderr: fmt.Sprintf("No events found in %s namespace.\n", ns) + namespaceHint(env, ns)}, nil
	}
	return Result{Stdout: renderTable(items, false, all, false, fl.boolean("no-headers"), env.State.Now)}, nil
}

// ---- api-resources / api-versions ----

func runAPIResources(args []string, env Env, p *Parsed) (Result, *Failure) {
	pos, fl, f := parseFlags(args, "api-resources", []flagDef{{Long: "namespaced", Value: true}, {Long: "output", Short: "o", Value: true}, {Long: "no-headers"}})
	if f != nil {
		return Result{}, f
	}
	if r, ok := helpRequested(fl, "api-resources"); ok {
		return r, nil
	}
	recordFlags(p, fl)
	if len(pos) > 0 {
		return Result{}, reject(CodeValidation, "error: unexpected argument %q", pos[0])
	}
	output := fl.str("output")
	if f := checkOutput(output, "wide", "name"); f != nil {
		return Result{}, f
	}
	var filter *bool
	if s := fl.str("namespaced"); s != "" {
		b, err := strconv.ParseBool(s)
		if err != nil {
			return Result{}, reject(CodeValidation, "error: invalid argument %q for \"--namespaced\" flag", s)
		}
		filter = &b
	}
	types := append([]resourceType{}, resourceTypes...)
	sort.SliceStable(types, func(i, j int) bool {
		if types[i].Group != types[j].Group {
			return types[i].Group < types[j].Group
		}
		return types[i].Plural < types[j].Plural
	})
	t := &table{header: []string{"NAME", "SHORTNAMES", "APIVERSION", "NAMESPACED", "KIND"}}
	if output == "wide" {
		t.header = append(t.header, "VERBS")
	}
	var names strings.Builder
	for _, rt := range types {
		if filter != nil && rt.Namespaced != *filter {
			continue
		}
		fmt.Fprintln(&names, rt.display())
		row := []string{rt.Plural, strings.Join(rt.Short, ","), rt.apiVersion(), strconv.FormatBool(rt.Namespaced), rt.Kind}
		if output == "wide" {
			verbs := "get,list"
			if rt.Kind != kindEvent && rt.Kind != kindEndpoints && rt.Kind != simulator.KindEndpointSlice && rt.Kind != simulator.KindNode {
				verbs = "create,delete,get,list,update"
			}
			row = append(row, "["+strings.ReplaceAll(verbs, ",", " ")+"]")
		}
		t.add(row...)
	}
	if output == "name" {
		return Result{Stdout: names.String()}, nil
	}
	return Result{
		Stdout: t.render(fl.boolean("no-headers")),
		Stderr: "ⓘ This simulated cluster serves only these resource types (see the support matrix).\n",
	}, nil
}

func runAPIVersions(args []string, _ Env, p *Parsed) (Result, *Failure) {
	_, fl, f := parseFlags(args, "api-versions", nil)
	if f != nil {
		return Result{}, f
	}
	if r, ok := helpRequested(fl, "api-versions"); ok {
		return r, nil
	}
	recordFlags(p, fl)
	return Result{Stdout: "apps/v1\ndiscovery.k8s.io/v1\nv1\n"}, nil
}

// ---- cluster-info / version ----

func runClusterInfo(args []string, _ Env, p *Parsed) (Result, *Failure) {
	pos, fl, f := parseFlags(args, "cluster-info", nil)
	if f != nil {
		return Result{}, f
	}
	if r, ok := helpRequested(fl, "cluster-info"); ok {
		return r, nil
	}
	recordFlags(p, fl)
	if len(pos) > 0 && pos[0] == "dump" {
		return Result{}, reject(CodeUnsupported, "error: 'cluster-info dump' isn't supported in the simulator; use 'kubectl get all -A' and 'kubectl events -A'")
	}
	if len(pos) > 0 {
		return Result{}, reject(CodeValidation, "error: unknown argument %q", pos[0])
	}
	return Result{Stdout: "Kubernetes control plane is running at " + serverURL + "\n\n" +
		"ⓘ This control plane is simulated by KubeDojo; there is no real API server at this address.\n"}, nil
}

func runVersion(args []string, _ Env, p *Parsed) (Result, *Failure) {
	_, fl, f := parseFlags(args, "version", []flagDef{{Long: "output", Short: "o", Value: true}, {Long: "client"}})
	if f != nil {
		return Result{}, f
	}
	if r, ok := helpRequested(fl, "version"); ok {
		return r, nil
	}
	recordFlags(p, fl)
	output := fl.str("output")
	if f := checkOutput(output, "json", "yaml"); f != nil {
		return Result{}, f
	}
	info := func(platform string) map[string]any {
		return map[string]any{"major": "1", "minor": "37", "gitVersion": clientVersion, "platform": platform}
	}
	if output != "" {
		v := map[string]any{"clientVersion": info("linux/amd64")}
		if !fl.boolean("client") {
			v["serverVersion"] = info("linux/amd64")
		}
		enc := marshalYAML
		if output == "json" {
			enc = marshalJSON
		}
		out, err := enc(v)
		if err != nil {
			return Result{}, reject(CodeValidation, "error: %s", err.Error())
		}
		return Result{Stdout: out}, nil
	}
	out := "Client Version: " + clientVersion + "\n"
	if !fl.boolean("client") {
		out += "Server Version: " + clientVersion + " (simulated)\n"
	}
	return Result{Stdout: out}, nil
}

// ---- config ----

func runConfig(args []string, env Env, p *Parsed) (Result, *Failure) {
	pos, fl, f := parseFlags(args, "config", []flagDef{{Long: "minify"}, {Long: "output", Short: "o", Value: true}, {Long: "raw"}, {Long: "current"}})
	if f != nil {
		return Result{}, f
	}
	if r, ok := helpRequested(fl, "config"); ok {
		return r, nil
	}
	recordFlags(p, fl)
	if len(pos) == 0 {
		return Result{Stdout: commandHelp["config"]}, nil
	}
	p.Subcommand = pos[0]
	ns := env.State.CurrentNamespace
	if ns == "" {
		ns = "default"
	}
	switch pos[0] {
	case "view":
		output := fl.str("output")
		if f := checkOutput(output, "json", "yaml"); f != nil {
			return Result{}, f
		}
		cfg := map[string]any{
			"apiVersion": "v1", "kind": "Config", "preferences": map[string]any{}, "current-context": contextName,
			"clusters": []any{map[string]any{"name": contextName, "cluster": map[string]any{"server": serverURL, "certificate-authority-data": "DATA+OMITTED"}}},
			"contexts": []any{map[string]any{"name": contextName, "context": map[string]any{"cluster": contextName, "user": "learner", "namespace": ns}}},
			"users":    []any{map[string]any{"name": "learner", "user": map[string]any{"client-certificate-data": "DATA+OMITTED", "client-key-data": "DATA+OMITTED"}}},
		}
		enc := marshalYAML
		if output == "json" {
			enc = marshalJSON
		}
		out, err := enc(cfg)
		if err != nil {
			return Result{}, reject(CodeValidation, "error: %s", err.Error())
		}
		return Result{Stdout: out}, nil
	case "get-contexts":
		t := &table{header: []string{"CURRENT", "NAME", "CLUSTER", "AUTHINFO", "NAMESPACE"}}
		t.add("*", contextName, contextName, "learner", ns)
		return Result{Stdout: t.render(false)}, nil
	case "current-context":
		return Result{Stdout: contextName + "\n"}, nil
	case "get-clusters":
		return Result{Stdout: "NAME\n" + contextName + "\n"}, nil
	case "get-users":
		return Result{Stdout: "NAME\nlearner\n"}, nil
	case "use-context", "use":
		if len(pos) < 2 || pos[1] != contextName {
			name := ""
			if len(pos) > 1 {
				name = pos[1]
			}
			return Result{}, reject(CodeNotFound, "error: no context exists with the name: %q", name)
		}
		return Result{Stdout: fmt.Sprintf("Switched to context %q.\n", contextName)}, nil
	case "set-context":
		return Result{}, reject(CodeUnsupported, "error: 'kubectl config set-context' changes your kubeconfig, which isn't available yet; use -n <namespace> on each command for now")
	}
	return Result{}, reject(CodeUnsupported, "error: 'kubectl config %s' isn't supported: the simulator has a single built-in context", pos[0])
}

// ---- rollout ----

func runRollout(args []string, env Env, p *Parsed) (Result, *Failure) {
	pos, fl, f := parseFlags(args, "rollout", []flagDef{{Long: "revision", Value: true}, {Long: "watch", Short: "w"}, {Long: "timeout", Value: true}})
	if f != nil {
		return Result{}, f
	}
	if r, ok := helpRequested(fl, "rollout"); ok {
		return r, nil
	}
	recordFlags(p, fl)
	if len(pos) == 0 {
		return Result{Stdout: commandHelp["rollout"]}, nil
	}
	sub := pos[0]
	p.Subcommand = sub
	switch sub {
	case "status", "history":
	case "undo", "restart", "pause", "resume":
		return Result{}, reject(CodeUnsupported, "error: 'kubectl rollout %s' changes the cluster, and changing the simulated cluster isn't available yet", sub)
	default:
		return Result{}, reject(CodeUnsupported, "error: unknown command %q for \"kubectl rollout\"", sub)
	}
	targets, f := resolveTargets(pos[1:], "rollout "+sub)
	if f != nil {
		return Result{}, f
	}
	if len(targets) != 1 || len(targets[0].names) != 1 {
		return Result{}, reject(CodeValidation, "error: specify one deployment, e.g. kubectl rollout %s deployment/web", sub)
	}
	tg := targets[0]
	if tg.t.Kind != simulator.KindDeployment {
		return Result{}, reject(CodeUnsupported, "error: no rollout %s for %s: the simulator supports rollouts of deployments only", sub, tg.t.display())
	}
	ns := namespaceFor(fl, env)
	name := tg.names[0]
	p.Namespace, p.Resources, p.Names = ns, []string{tg.t.Kind}, []string{name}
	d, ok := env.State.Get(simulator.KindDeployment, ns, name).(*simulator.Deployment)
	if !ok {
		return Result{}, reject(CodeNotFound, "%s", strings.TrimSuffix(notFound(tg.t, name), "\n"))
	}
	if sub == "status" {
		return rolloutStatus(d), nil
	}
	return rolloutHistory(env, d, fl.str("revision"))
}

func rolloutStatus(d *simulator.Deployment) Result {
	s, name, desired := d.Status, d.Metadata.Name, d.DesiredReplicas()
	var waiting string
	switch {
	case s.UpdatedReplicas < desired:
		waiting = fmt.Sprintf("Waiting for deployment %q rollout to finish: %d out of %d new replicas have been updated...", name, s.UpdatedReplicas, desired)
	case s.Replicas > s.UpdatedReplicas:
		waiting = fmt.Sprintf("Waiting for deployment %q rollout to finish: %d old replicas are pending termination...", name, s.Replicas-s.UpdatedReplicas)
	case s.AvailableReplicas < s.UpdatedReplicas:
		waiting = fmt.Sprintf("Waiting for deployment %q rollout to finish: %d of %d updated replicas are available...", name, s.AvailableReplicas, s.UpdatedReplicas)
	default:
		return Result{Stdout: fmt.Sprintf("deployment %q successfully rolled out\n", name)}
	}
	// A real kubectl would keep waiting. The simulated cluster is already reconciled, so the
	// rollout can't progress until something changes: say so instead of hanging.
	return Result{
		Stdout:   waiting + "\n",
		Stderr:   "ⓘ The simulated cluster is fully reconciled, so this rollout won't progress until something changes. Look at the Pods: kubectl get pods -l " + selectorString(d.Spec.Selector) + "\n",
		ExitCode: 1,
	}
}

func rolloutHistory(env Env, d *simulator.Deployment, revision string) (Result, *Failure) {
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
		n, err := strconv.ParseInt(rs.Metadata.Annotations[simulator.RevisionAnnotation], 10, 64)
		if err == nil {
			revs = append(revs, rev{n, rs})
		}
	}
	sort.Slice(revs, func(i, j int) bool { return revs[i].n < revs[j].n })
	header := "deployment.apps/" + d.Metadata.Name
	if revision != "" {
		want, err := strconv.ParseInt(revision, 10, 64)
		if err != nil || want < 0 {
			return Result{}, reject(CodeValidation, "error: invalid argument %q for \"--revision\" flag", revision)
		}
		if want == 0 && len(revs) > 0 {
			want = revs[len(revs)-1].n
		}
		for _, r := range revs {
			if r.n == want {
				dsc := &describer{now: env.State.Now}
				tmpl := r.rs.Spec.Template
				dsc.podTemplate(tmpl)
				return Result{Stdout: fmt.Sprintf("%s with revision #%d\n", header, want) + dsc.String()}, nil
			}
		}
		return Result{}, reject(CodeNotFound, "error: unable to find the specified revision")
	}
	t := &table{header: []string{"REVISION", "CHANGE-CAUSE"}, pad: 2}
	for _, r := range revs {
		t.add(strconv.FormatInt(r.n, 10), orNone(r.rs.Metadata.Annotations["kubernetes.io/change-cause"]))
	}
	return Result{Stdout: header + "\n" + t.render(false) + "\n"}, nil
}
