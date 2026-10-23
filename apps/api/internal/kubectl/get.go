package kubectl

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// item is one listed resource: a simulator object, an event, or a derived Endpoints view.
type item struct {
	t         resourceType
	ns, name  string
	display   string // NAME column (prefixed with the type when several types are listed)
	labels    map[string]string
	created   time.Time
	obj       simulator.Object
	event     *simulator.Event
	endpoints []string // Endpoints view: "ip:port" of ready endpoints
}

// generic returns the item as kubectl would print it with -o yaml/json.
func (it item) generic() (any, error) {
	switch {
	case it.obj != nil:
		return toGeneric(it.obj)
	case it.event != nil:
		e := it.event
		ns := e.Namespace
		if ns == "" {
			ns = "default"
		}
		return map[string]any{
			"apiVersion": "v1", "kind": "Event", "type": e.Type, "reason": e.Reason, "message": e.Message,
			"count": 1, "firstTimestamp": e.Time.Format(time.RFC3339), "lastTimestamp": e.Time.Format(time.RFC3339),
			"involvedObject": map[string]any{"kind": e.Kind, "name": e.Name, "namespace": e.Namespace},
			"metadata": map[string]any{
				"name": it.name, "namespace": ns, "creationTimestamp": e.Time.Format(time.RFC3339),
				"annotations": map[string]any{simulator.SimulatedAnnotation: "true"},
			},
			"source": map[string]any{"component": eventSource(e.Reason)},
		}, nil
	default:
		var subsets []any
		if len(it.endpoints) > 0 {
			var addrs []any
			ports := map[string]bool{}
			for _, ep := range it.endpoints {
				ip, port, _ := strings.Cut(ep, ":")
				addrs = append(addrs, map[string]any{"ip": ip})
				ports[port] = true
			}
			var ps []any
			for p := range ports {
				var n int
				_, _ = fmt.Sscan(p, &n)
				ps = append(ps, map[string]any{"port": n, "protocol": "TCP"})
			}
			subsets = []any{map[string]any{"addresses": addrs, "ports": ps}}
		}
		out := map[string]any{
			"apiVersion": "v1", "kind": "Endpoints",
			"metadata": map[string]any{
				"name": it.name, "namespace": it.ns, "creationTimestamp": it.created.Format(time.RFC3339),
				"annotations": map[string]any{simulator.SimulatedAnnotation: "true"},
			},
		}
		if subsets != nil {
			out["subsets"] = subsets
		}
		return out, nil
	}
}

// eventSource names the component that records an event reason, for describe's From column.
func eventSource(reason string) string {
	switch reason {
	case "Scheduled", "FailedScheduling":
		return "default-scheduler"
	case "ScalingReplicaSet":
		return "deployment-controller"
	case "SuccessfulCreate", "SuccessfulDelete", "FailedCreate":
		return "replicaset-controller"
	}
	return "kubelet"
}

// allEvents returns stored and new events in time order.
func allEvents(env Env) []simulator.Event {
	evs := append(append([]simulator.Event{}, env.Events...), env.State.Events()...)
	sort.SliceStable(evs, func(i, j int) bool { return evs[i].Time.Before(evs[j].Time) })
	return evs
}

// listItems returns every item of type t in scope (namespace ns, or all namespaces).
func listItems(env Env, t resourceType, ns string, all bool) []item {
	st := env.State
	scope := ns
	if all || !t.Namespaced {
		scope = "*"
	}
	var out []item
	switch t.Kind {
	case kindEvent:
		for i, e := range allEvents(env) {
			e := e
			evNS := e.Namespace
			if evNS == "" {
				evNS = "default" // events about cluster-scoped objects live in default
			}
			if scope != "*" && evNS != scope {
				continue
			}
			name := fmt.Sprintf("%s.%x", e.Name, e.Time.UnixNano()+int64(i))
			out = append(out, item{t: t, ns: evNS, name: name, display: name, created: e.Time, event: &e})
		}
		return out
	case kindEndpoints:
		for _, o := range st.List(simulator.KindService, scope) {
			svc := o.(*simulator.Service)
			out = append(out, item{
				t: t, ns: svc.Metadata.Namespace, name: svc.Metadata.Name, display: svc.Metadata.Name,
				labels: svc.Metadata.Labels, created: svc.Metadata.CreationTimestamp, endpoints: endpointsOf(st, svc),
			})
		}
		return out
	}
	for _, o := range st.List(t.Kind, scope) {
		m := o.Meta()
		out = append(out, item{t: t, ns: m.Namespace, name: m.Name, display: m.Name, labels: m.Labels, created: m.CreationTimestamp, obj: o})
	}
	return out
}

// endpointsOf derives the v1 Endpoints view of a Service from its EndpointSlice. The API
// server's own Service points at the control plane.
func endpointsOf(st *simulator.State, svc *simulator.Service) []string {
	if svc.Metadata.Name == "kubernetes" && svc.Metadata.Namespace == "default" {
		for _, o := range st.List(simulator.KindNode, "*") {
			n := o.(*simulator.Node)
			if _, cp := n.Metadata.Labels["node-role.kubernetes.io/control-plane"]; cp {
				if ip := nodeAddress(n, "InternalIP"); ip != "<none>" {
					return []string{ip + ":6443"}
				}
			}
		}
		return nil
	}
	var out []string
	for _, o := range st.List(simulator.KindEndpointSlice, svc.Metadata.Namespace) {
		s := o.(*simulator.EndpointSlice)
		if s.Metadata.Labels["kubernetes.io/service-name"] != svc.Metadata.Name {
			continue
		}
		for _, e := range s.Endpoints {
			if !e.Conditions.Ready {
				continue
			}
			for _, a := range e.Addresses {
				for _, p := range s.Ports {
					out = append(out, fmt.Sprintf("%s:%d", a, p.Port))
				}
			}
		}
	}
	return out
}

type target struct {
	t     resourceType
	names []string
}

var allTypes = []string{"pods", "services", "deployments", "replicasets"}

// resolveTargets reads `TYPE[,TYPE…] [NAME…]` or `TYPE/NAME…` arguments.
func resolveTargets(pos []string, verb string) ([]target, *Failure) {
	if len(pos) == 0 {
		return nil, reject(CodeValidation, "You must specify the type of resource to %s. Use \"kubectl api-resources\" for a complete list of supported resources.\n\nerror: Required resource not specified.", verb)
	}
	if strings.Contains(pos[0], "/") {
		var out []target
		for _, a := range pos {
			typ, name, ok := strings.Cut(a, "/")
			if !ok {
				return nil, reject(CodeValidation, "error: there is no need to specify a resource type as a separate argument when passing arguments in resource/name form (e.g. 'kubectl %s resource/<resource_name>' instead of 'kubectl %s resource resource/<resource_name>'", verb, verb)
			}
			t, f := lookupType(typ)
			if f != nil {
				return nil, f
			}
			if name == "" {
				return nil, reject(CodeValidation, "error: arguments in resource/name form must have a single resource and name")
			}
			out = append(out, target{t: t, names: []string{name}})
		}
		return out, nil
	}
	for _, a := range pos[1:] {
		if strings.Contains(a, "/") {
			return nil, reject(CodeValidation, "error: there is no need to specify a resource type as a separate argument when passing arguments in resource/name form (e.g. 'kubectl %s resource/<resource_name>' instead of 'kubectl %s resource resource/<resource_name>'", verb, verb)
		}
	}
	var types []string
	for _, typ := range strings.Split(pos[0], ",") {
		if typ == "all" {
			types = append(types, allTypes...)
		} else if typ != "" {
			types = append(types, typ)
		}
	}
	var out []target
	seen := map[string]bool{}
	for _, typ := range types {
		t, f := lookupType(typ)
		if f != nil {
			return nil, f
		}
		if seen[t.Kind] {
			continue
		}
		seen[t.Kind] = true
		out = append(out, target{t: t, names: pos[1:]})
	}
	return out, nil
}

var getFlags = []flagDef{
	{Long: "output", Short: "o", Value: true},
	{Long: "selector", Short: "l", Value: true},
	{Long: "show-labels"},
	{Long: "no-headers"},
	{Long: "sort-by", Value: true},
}

func checkOutput(o string, allowed ...string) *Failure {
	if o == "" {
		return nil
	}
	for _, a := range allowed {
		if o == a {
			return nil
		}
	}
	for _, unsupported := range []string{"jsonpath", "custom-columns", "go-template", "template"} {
		if strings.HasPrefix(o, unsupported) {
			return reject(CodeUnsupported, "error: -o %s isn't supported in the simulator; use one of: %s", unsupported, strings.Join(allowed, ", "))
		}
	}
	return reject(CodeValidation, "error: unable to match a printer suitable for the output format %q, allowed formats are: %s", o, strings.Join(allowed, ","))
}

func namespaceExists(env Env, ns string) bool {
	return env.State.Has(simulator.KindNamespace, "", ns)
}

func namespaceHint(env Env, ns string) string {
	if namespaceExists(env, ns) {
		return ""
	}
	return fmt.Sprintf("ⓘ The namespace %q doesn't exist. List namespaces with: kubectl get namespaces\n", ns)
}

func notFound(t resourceType, name string) string {
	return fmt.Sprintf("Error from server (NotFound): %s %q not found\n", t.display(), name)
}

// selectItems applies names (reporting missing ones) and the label selector.
func selectItems(env Env, tg target, ns string, all bool, sel simulator.Selector) (found []item, missing string) {
	items := listItems(env, tg.t, ns, all)
	if len(tg.names) == 0 {
		for _, it := range items {
			if sel.Matches(it.labels) {
				found = append(found, it)
			}
		}
		return found, ""
	}
	byName := map[string]item{}
	for _, it := range items {
		byName[it.name] = it
	}
	var errs strings.Builder
	for _, n := range tg.names {
		it, ok := byName[n]
		if !ok {
			errs.WriteString(notFound(tg.t, n))
			continue
		}
		if sel.Matches(it.labels) {
			found = append(found, it)
		}
	}
	return found, errs.String()
}

func runGet(args []string, env Env, p *Parsed) (Result, *Failure) {
	pos, fl, f := parseFlags(args, "get", getFlags)
	if f != nil {
		return Result{}, f
	}
	if fl.boolean("help") {
		return Result{Stdout: commandHelp["get"]}, nil
	}
	recordFlags(p, fl)
	output := fl.str("output")
	if f := checkOutput(output, "json", "name", "wide", "yaml"); f != nil {
		return Result{}, f
	}
	sel, err := simulator.ParseSelector(fl.str("selector"))
	if err != nil {
		return Result{}, reject(CodeValidation, "error: %s", err.Error())
	}
	sortBy := fl.str("sort-by")
	if sortBy != "" && sortBy != "metadata.name" && sortBy != ".metadata.name" && sortBy != "metadata.creationTimestamp" && sortBy != ".metadata.creationTimestamp" {
		return Result{}, reject(CodeUnsupported, "error: --sort-by %s isn't supported in the simulator; use metadata.name or metadata.creationTimestamp", sortBy)
	}
	targets, f := resolveTargets(pos, "get")
	if f != nil {
		return Result{}, f
	}
	ns, all := namespaceFor(fl, env), fl.boolean("all-namespaces")
	p.Namespace, p.AllNamespaces, p.Output, p.Selector = ns, all, output, fl.str("selector")
	for _, tg := range targets {
		p.Resources = append(p.Resources, tg.t.Kind)
		p.Names = append(p.Names, tg.names...)
		if all && len(tg.names) > 0 {
			return Result{}, reject(CodeValidation, "error: a resource cannot be retrieved by name across all namespaces")
		}
	}

	multi := len(targets) > 1
	var stderr strings.Builder
	var groups [][]item
	anyNamespaced := false
	for _, tg := range targets {
		if tg.t.Namespaced {
			anyNamespaced = true
		}
		found, missing := selectItems(env, tg, ns, all, sel)
		stderr.WriteString(missing)
		if missing != "" && tg.t.Namespaced && !all {
			stderr.WriteString(namespaceHint(env, ns))
		}
		if len(tg.names) == 0 || sortBy != "" {
			sortItems(found, sortBy, all) // named resources print in the order requested
		}
		for i := range found {
			if multi {
				found[i].display = tg.t.qualified() + "/" + found[i].name
			}
		}
		groups = append(groups, found)
	}
	if tg := targets[0]; tg.t.Kind == kindEndpoints {
		stderr.WriteString("Warning: v1 Endpoints is deprecated in v1.33+; use discovery.k8s.io/v1 EndpointSlice\n")
	}

	var flat []item
	for _, g := range groups {
		flat = append(flat, g...)
	}
	res := Result{}
	switch output {
	case "name":
		var b strings.Builder
		for _, it := range flat {
			fmt.Fprintf(&b, "%s/%s\n", it.t.qualified(), it.name)
		}
		res.Stdout = b.String()
	case "yaml", "json":
		single := len(targets) == 1 && len(targets[0].names) == 1 && len(flat) == 1
		out, redacted, err := printStructured(flat, single, output)
		if err != nil {
			return Result{}, reject(CodeValidation, "error: %s", err.Error())
		}
		res.Stdout, res.Redacted = out, redacted
	default:
		var blocks []string
		for _, g := range groups {
			if len(g) > 0 {
				blocks = append(blocks, renderTable(g, output == "wide", all, fl.boolean("show-labels"), fl.boolean("no-headers"), env.State.Now))
			}
		}
		res.Stdout = strings.Join(blocks, "\n")
	}
	if len(flat) == 0 && !strings.Contains(stderr.String(), "NotFound") {
		if output == "" || output == "wide" {
			switch {
			case all || !anyNamespaced:
				stderr.WriteString("No resources found\n")
			default:
				fmt.Fprintf(&stderr, "No resources found in %s namespace.\n", ns)
				stderr.WriteString(namespaceHint(env, ns))
			}
		}
	}
	res.Stderr = stderr.String()
	if strings.Contains(res.Stderr, "Error from server (NotFound)") {
		res.ExitCode, res.ErrorCode = 1, CodeNotFound
	}
	return res, nil
}

func sortItems(items []item, sortBy string, all bool) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if strings.HasSuffix(sortBy, "creationTimestamp") && !a.created.Equal(b.created) {
			return a.created.Before(b.created)
		}
		if a.t.Kind == kindEvent && sortBy == "" {
			return a.created.Before(b.created)
		}
		if all && a.ns != b.ns && !strings.HasSuffix(sortBy, "metadata.name") {
			return a.ns < b.ns
		}
		return a.name < b.name
	})
}

func renderTable(items []item, wide, all, showLabels, noHeaders bool, now time.Time) string {
	t := &table{}
	namespaced := items[0].t.Namespaced
	for _, it := range items {
		header, row := columns(it, wide, now)
		if all && namespaced {
			header = append([]string{"NAMESPACE"}, header...)
			row = append([]string{it.ns}, row...)
		}
		if showLabels && it.t.Kind != kindEvent {
			header = append(header, "LABELS")
			row = append(row, labelString(it.labels))
		}
		t.header = header
		t.add(row...)
	}
	return t.render(noHeaders)
}

// printStructured renders items as YAML or JSON, plus a copy with Secret values redacted.
func printStructured(items []item, single bool, format string) (string, string, error) {
	var plain, redacted []any
	for _, it := range items {
		g, err := it.generic()
		if err != nil {
			return "", "", err
		}
		plain = append(plain, g)
		r, err := it.generic()
		if err != nil {
			return "", "", err
		}
		if it.t.Kind == simulator.KindSecret {
			redactSecret(r)
		}
		redacted = append(redacted, r)
	}
	wrap := func(v []any) any {
		if single {
			return v[0]
		}
		if v == nil {
			v = []any{}
		}
		return listOf(v)
	}
	enc := marshalYAML
	if format == "json" {
		enc = marshalJSON
	}
	out, err := enc(wrap(plain))
	if err != nil {
		return "", "", err
	}
	red, err := enc(wrap(redacted))
	return out, red, err
}

// redactSecret replaces Secret values so command history never keeps them.
func redactSecret(v any) {
	m, ok := v.(map[string]any)
	if !ok {
		return
	}
	if data, ok := m["data"].(map[string]any); ok {
		for k := range data {
			data[k] = "<redacted>"
		}
	}
}
