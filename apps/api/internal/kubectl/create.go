package kubectl

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// ---- run ----

var runFlags = append([]flagDef{
	{Long: "image", Value: true},
	{Long: "labels", Short: "l", Value: true},
	{Long: "annotations", Value: true},
	{Long: "env", Value: true},
	{Long: "port", Value: true},
	{Long: "restart", Value: true},
	{Long: "rm"},
	{Long: "stdin", Short: "i"},
	{Long: "tty", Short: "t"},
	{Long: "attach"},
	{Long: "command"},
	{Long: "expose"},
	{Long: "overrides", Value: true},
	{Long: "image-pull-policy", Value: true},
}, dryRunFlags...)

func runRun(args []string, env Env, p *Parsed) (Result, *Failure) {
	for _, a := range args {
		if a == "-it" || a == "-ti" {
			return Result{}, reject(CodeUnsupported, "error: -it isn't supported: no containers actually run in the simulator, so there is no shell to attach to. Create the Pod, then use kubectl logs and kubectl describe.")
		}
	}
	pos, fl, f := parseFlags(args, "run", runFlags)
	if f != nil {
		return Result{}, f
	}
	if r, ok := helpRequested(fl, "run"); ok {
		return r, nil
	}
	recordFlags(p, fl)
	for _, flag := range []string{"rm", "stdin", "tty", "attach"} {
		if fl.boolean(flag) {
			return Result{}, reject(CodeUnsupported, "error: --%s isn't supported: no containers actually run in the simulator, so there is nothing to attach to. Create the Pod, then use kubectl logs and kubectl describe.", flag)
		}
	}
	switch {
	case fl.boolean("command") || len(pos) > 1:
		return Result{}, reject(CodeUnsupported, "error: container commands and arguments (--command, -- ARGS) aren't simulated: each image has a fixed behavior. Leave them out.")
	case fl.boolean("expose"):
		return Result{}, reject(CodeUnsupported, "error: --expose isn't supported in the simulator; create the Pod, then run kubectl expose pod <name> --port=<port>")
	case fl.str("overrides") != "":
		return Result{}, reject(CodeUnsupported, "error: --overrides isn't supported; write a manifest in the lab editor and use kubectl apply -f")
	case fl.str("image-pull-policy") != "":
		return Result{}, reject(CodeUnsupported, "error: --image-pull-policy isn't simulated: images resolve against the lab's image catalog")
	}
	if restart := fl.str("restart"); restart != "" && restart != "Always" {
		return Result{}, reject(CodeUnsupported, "error: --restart=%s isn't simulated: Pods in KubeDojo always restart their containers (--restart=Always)", restart)
	}
	if len(pos) == 0 {
		return Result{}, reject(CodeValidation, "error: NAME is required for run\nSee 'kubectl run -h' for help and examples")
	}
	name, image := pos[0], fl.str("image")
	if image == "" {
		return Result{}, reject(CodeValidation, "error: required flag(s) \"image\" not set")
	}
	dry, f := dryRun(fl)
	if f != nil {
		return Result{}, f
	}
	labels := map[string]string{"run": name}
	if s := fl.str("labels"); s != "" {
		if labels, f = splitLabels(s); f != nil {
			return Result{}, f
		}
	}
	var annotations map[string]string
	if v := fl.values["annotations"]; len(v) > 0 {
		if annotations, _, f = keyValues(v, "annotation"); f != nil {
			return Result{}, f
		}
	}
	c := simulator.Container{Name: name, Image: image}
	envs, order, f := keyValues(fl.values["env"], "env")
	if f != nil {
		return Result{}, f
	}
	for _, k := range order {
		c.Env = append(c.Env, simulator.EnvVar{Name: k, Value: envs[k]})
	}
	if s := fl.str("port"); s != "" {
		port, f := parsePort(s, "port")
		if f != nil {
			return Result{}, f
		}
		c.Ports = []simulator.ContainerPort{{ContainerPort: port}}
	}
	ns := namespaceFor(fl, env)
	p.Namespace, p.Resources, p.Names = ns, []string{simulator.KindPod}, []string{name}
	pod := &simulator.Pod{
		Metadata: simulator.ObjectMeta{Name: name, Namespace: ns, Labels: labels, Annotations: annotations},
		Spec:     simulator.PodSpec{Containers: []simulator.Container{c}, RestartPolicy: "Always"},
	}
	return created(env, pod, "created", fl.str("output"), dry)
}

func parsePort(s, flag string) (int32, *Failure) {
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil || n < 1 || n > 65535 {
		return 0, reject(CodeValidation, "error: invalid argument %q for \"--%s\" flag: must be a port number between 1 and 65535", s, flag)
	}
	return int32(n), nil
}

// ---- create ----

var createFlags = append([]flagDef{
	{Long: "image", Value: true},
	{Long: "replicas", Short: "r", Value: true},
	{Long: "port", Value: true},
	{Long: "from-literal", Value: true},
	{Long: "from-file", Value: true},
	{Long: "from-env-file", Value: true},
	{Long: "tcp", Value: true},
	{Long: "node-port", Value: true},
	{Long: "clusterip", Value: true},
	{Long: "type", Value: true},
	{Long: "filename", Short: "f", Value: true},
	{Long: "save-config"},
}, dryRunFlags...)

// createUnsupported are `kubectl create` subcommands for kinds the simulator doesn't have.
var createUnsupported = map[string]string{
	"job": "Jobs", "cronjob": "CronJobs", "cj": "CronJobs", "ingress": "Ingresses", "ing": "Ingresses",
	"role": "Roles", "rolebinding": "RoleBindings", "clusterrole": "ClusterRoles", "clusterrolebinding": "ClusterRoleBindings",
	"serviceaccount": "ServiceAccounts", "sa": "ServiceAccounts", "quota": "ResourceQuotas", "resourcequota": "ResourceQuotas",
	"priorityclass": "PriorityClasses", "pc": "PriorityClasses", "poddisruptionbudget": "PodDisruptionBudgets", "pdb": "PodDisruptionBudgets",
	"token": "ServiceAccount tokens",
}

func runCreate(args []string, env Env, p *Parsed) (Result, *Failure) {
	// Unsupported kinds have flags this parser doesn't know (--schedule): name the kind first.
	for i := 0; i < len(args); i++ {
		if args[i] == "-n" || args[i] == "--namespace" {
			i++
			continue
		}
		if strings.HasPrefix(args[i], "-") {
			continue
		}
		if what, ok := createUnsupported[args[i]]; ok {
			p.Subcommand = args[i]
			return Result{}, reject(CodeUnsupported, "error: %s are not part of the KubeDojo simulator yet. Run 'kubectl api-resources' to see what is.", what)
		}
		break
	}
	pos, fl, f := parseFlags(args, "create", createFlags)
	if f != nil {
		return Result{}, f
	}
	if r, ok := helpRequested(fl, "create"); ok {
		return r, nil
	}
	recordFlags(p, fl)
	if fl.str("filename") != "" {
		return createFromFile(fl, env, p, pos)
	}
	if len(pos) == 0 {
		return Result{}, reject(CodeValidation, "error: must specify one of -f and -k, or a resource to create, e.g. kubectl create deployment web --image=nginx:1.27\nSee 'kubectl create -h' for help and examples")
	}
	sub := pos[0]
	p.Subcommand = sub
	if what, ok := createUnsupported[sub]; ok {
		return Result{}, reject(CodeUnsupported, "error: %s are not part of the KubeDojo simulator yet. Run 'kubectl api-resources' to see what is.", what)
	}
	dry, f := dryRun(fl)
	if f != nil {
		return Result{}, f
	}
	ns := namespaceFor(fl, env)
	rest := pos[1:]
	var obj simulator.Object
	switch sub {
	case "deployment", "deploy":
		obj, f = newDeployment(rest, fl, ns)
	case "namespace", "ns":
		obj, f = newNamespace(rest)
	case "configmap", "cm":
		obj, f = newConfigMap(rest, fl, ns)
	case "secret":
		obj, f = newSecret(rest, fl, ns)
	case "service", "svc":
		obj, f = newService(rest, fl, ns)
	default:
		return Result{}, reject(CodeUnsupported, "error: unknown or unsupported resource %q for 'kubectl create'. The simulator can create: deployment, namespace, configmap, secret generic, service clusterip|nodeport|loadbalancer.", truncateWord(sub))
	}
	if f != nil {
		return Result{}, f
	}
	p.Namespace, p.Resources, p.Names = ns, []string{obj.GetKind()}, []string{obj.Meta().Name}
	if simulator.ClusterScoped(obj.GetKind()) {
		p.Namespace = ""
	}
	res, f := created(env, obj, "created", fl.str("output"), dry)
	if obj.GetKind() == simulator.KindSecret && f == nil {
		res.Stderr += "ⓘ Simulated Secrets are for practice. Never put real passwords, tokens or keys in a lab.\n"
	}
	return res, f
}

func newDeployment(pos []string, fl flags, ns string) (simulator.Object, *Failure) {
	name, f := requireName(pos, "kubectl create deployment NAME --image=IMAGE [--replicas=N] [--port=PORT]")
	if f != nil {
		return nil, f
	}
	images := fl.values["image"]
	if len(images) == 0 {
		return nil, reject(CodeValidation, "error: required flag(s) \"image\" not set")
	}
	replicas := int32(1)
	if s := fl.str("replicas"); s != "" {
		n, err := strconv.ParseInt(s, 10, 32)
		if err != nil || n < 0 {
			return nil, reject(CodeValidation, "error: invalid argument %q for \"--replicas\" flag: must be a whole number of 0 or more", s)
		}
		replicas = int32(n)
	}
	var containers []simulator.Container
	used := map[string]int{}
	for _, img := range images {
		cname := containerName(img)
		if n := used[cname]; n > 0 {
			cname = fmt.Sprintf("%s-%d", cname, n)
		}
		used[containerName(img)]++
		containers = append(containers, simulator.Container{Name: cname, Image: img})
	}
	if s := fl.str("port"); s != "" {
		port, f := parsePort(s, "port")
		if f != nil {
			return nil, f
		}
		containers[0].Ports = []simulator.ContainerPort{{ContainerPort: port}}
	}
	labels := map[string]string{"app": name}
	return &simulator.Deployment{
		Metadata: simulator.ObjectMeta{Name: name, Namespace: ns, Labels: copyMap(labels)},
		Spec: simulator.DeploymentSpec{
			Replicas: &replicas,
			Selector: simulator.LabelSelector{MatchLabels: copyMap(labels)},
			Template: simulator.PodTemplateSpec{
				Metadata: simulator.TemplateMeta{Labels: copyMap(labels)},
				Spec:     simulator.PodSpec{Containers: containers},
			},
		},
	}, nil
}

func copyMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func newNamespace(pos []string) (simulator.Object, *Failure) {
	name, f := requireName(pos, "kubectl create namespace NAME")
	if f != nil {
		return nil, f
	}
	return &simulator.Namespace{Metadata: simulator.ObjectMeta{Name: name}}, nil
}

// literalData reads --from-literal pairs, refusing duplicate keys as kubectl does.
func literalData(fl flags, kind, name string) (map[string]string, *Failure) {
	if fl.str("from-file") != "" || fl.str("from-env-file") != "" {
		return nil, reject(CodeUnsupported, "error: --from-file and --from-env-file need files; the lab has none to read yet. Use --from-literal=KEY=VALUE.")
	}
	data := map[string]string{}
	for _, kv := range fl.values["from-literal"] {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return nil, reject(CodeValidation, "error: invalid literal source %s, expected key=value", kv)
		}
		if _, dup := data[k]; dup {
			return nil, reject(CodeValidation, "error: cannot add key %q, another key by that name already exists in Data for %s %q", k, kind, name)
		}
		data[k] = v
	}
	return data, nil
}

func newConfigMap(pos []string, fl flags, ns string) (simulator.Object, *Failure) {
	name, f := requireName(pos, "kubectl create configmap NAME --from-literal=KEY=VALUE")
	if f != nil {
		return nil, f
	}
	data, f := literalData(fl, "ConfigMap", name)
	if f != nil {
		return nil, f
	}
	cm := &simulator.ConfigMap{Metadata: simulator.ObjectMeta{Name: name, Namespace: ns}}
	if len(data) > 0 {
		cm.Data = data
	}
	return cm, nil
}

func newSecret(pos []string, fl flags, ns string) (simulator.Object, *Failure) {
	if len(pos) == 0 {
		return nil, reject(CodeValidation, "error: specify the type of Secret: kubectl create secret generic NAME --from-literal=KEY=VALUE")
	}
	switch pos[0] {
	case "generic":
	case "docker-registry", "tls":
		return nil, reject(CodeUnsupported, "error: %s Secrets aren't simulated; KubeDojo has Opaque Secrets only (kubectl create secret generic)", pos[0])
	default:
		return nil, reject(CodeValidation, "error: unknown Secret type %q; use: kubectl create secret generic NAME --from-literal=KEY=VALUE", truncateWord(pos[0]))
	}
	if t := fl.str("type"); t != "" && t != "Opaque" {
		return nil, reject(CodeUnsupported, "error: --type=%s isn't simulated; KubeDojo has Opaque Secrets only", t)
	}
	name, f := requireName(pos[1:], "kubectl create secret generic NAME --from-literal=KEY=VALUE")
	if f != nil {
		return nil, f
	}
	data, f := literalData(fl, "Secret", name)
	if f != nil {
		return nil, f
	}
	s := &simulator.Secret{Metadata: simulator.ObjectMeta{Name: name, Namespace: ns}, Type: "Opaque"}
	for k, v := range data {
		if s.Data == nil {
			s.Data = map[string][]byte{}
		}
		s.Data[k] = []byte(v)
	}
	return s, nil
}

func newService(pos []string, fl flags, ns string) (simulator.Object, *Failure) {
	if len(pos) == 0 {
		return nil, reject(CodeValidation, "error: specify the type of Service: kubectl create service clusterip|nodeport|loadbalancer NAME --tcp=PORT:TARGET_PORT")
	}
	var typ string
	switch pos[0] {
	case "clusterip":
		typ = "ClusterIP"
	case "nodeport":
		typ = "NodePort"
	case "loadbalancer":
		typ = "LoadBalancer"
	case "externalname":
		return nil, reject(CodeUnsupported, "error: ExternalName Services aren't simulated; use clusterip or nodeport")
	default:
		return nil, reject(CodeValidation, "error: unknown Service type %q; use clusterip, nodeport or loadbalancer", truncateWord(pos[0]))
	}
	name, f := requireName(pos[1:], "kubectl create service "+pos[0]+" NAME --tcp=PORT:TARGET_PORT")
	if f != nil {
		return nil, f
	}
	if ip := fl.str("clusterip"); ip != "" {
		return nil, reject(CodeUnsupported, "error: --clusterip isn't supported: the simulator assigns ClusterIPs, and headless Services aren't simulated")
	}
	specs := fl.values["tcp"]
	if len(specs) == 0 {
		return nil, reject(CodeValidation, "error: at least one tcp port specifier must be provided, e.g. --tcp=80:8080")
	}
	var ports []simulator.ServicePort
	for _, spec := range specs {
		for _, one := range strings.Split(spec, ",") {
			ps, ts, hasTarget := strings.Cut(one, ":")
			port, f := parsePort(ps, "tcp")
			if f != nil {
				return nil, f
			}
			target := port
			if hasTarget {
				if target, f = parsePort(ts, "tcp"); f != nil {
					return nil, f
				}
			}
			ports = append(ports, simulator.ServicePort{
				Name: fmt.Sprintf("%d-%d", port, target), Port: port, Protocol: "TCP",
				TargetPort: simulator.IntOrString{IntVal: target},
			})
		}
	}
	if s := fl.str("node-port"); s != "" {
		if typ != "NodePort" {
			return nil, reject(CodeValidation, "error: --node-port can only be used with kubectl create service nodeport")
		}
		n, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return nil, reject(CodeValidation, "error: invalid argument %q for \"--node-port\" flag", s)
		}
		ports[0].NodePort = int32(n)
	}
	return &simulator.Service{
		Metadata: simulator.ObjectMeta{Name: name, Namespace: ns, Labels: map[string]string{"app": name}},
		Spec:     simulator.ServiceSpec{Type: typ, Selector: map[string]string{"app": name}, Ports: ports},
	}, nil
}

// ---- expose ----

var exposeFlags = append([]flagDef{
	{Long: "port", Value: true},
	{Long: "target-port", Value: true},
	{Long: "protocol", Value: true},
	{Long: "type", Value: true},
	{Long: "name", Value: true},
	{Long: "selector", Value: true},
	{Long: "labels", Short: "l", Value: true},
	{Long: "filename", Short: "f", Value: true},
	{Long: "external-ip", Value: true},
	{Long: "cluster-ip", Value: true},
}, dryRunFlags...)

func runExpose(args []string, env Env, p *Parsed) (Result, *Failure) {
	pos, fl, f := parseFlags(args, "expose", exposeFlags)
	if f != nil {
		return Result{}, f
	}
	if r, ok := helpRequested(fl, "expose"); ok {
		return r, nil
	}
	recordFlags(p, fl)
	switch {
	case fl.str("filename") != "":
		return Result{}, reject(CodeUnsupported, "error: expose -f isn't supported; use kubectl expose TYPE NAME")
	case fl.str("external-ip") != "" || fl.str("cluster-ip") != "":
		return Result{}, reject(CodeUnsupported, "error: --external-ip and --cluster-ip aren't simulated: the simulator assigns Service addresses")
	}
	if proto := fl.str("protocol"); proto != "" && proto != "TCP" {
		return Result{}, reject(CodeUnsupported, "error: --protocol=%s isn't simulated; Services in KubeDojo use TCP", proto)
	}
	targets, f := resolveTargets(pos, "expose")
	if f != nil {
		return Result{}, f
	}
	if len(targets) != 1 || len(targets[0].names) != 1 {
		return Result{}, reject(CodeValidation, "error: expose takes exactly one resource, e.g. kubectl expose deployment web --port=80")
	}
	tg := targets[0]
	ns := namespaceFor(fl, env)
	name := tg.names[0]
	p.Namespace, p.Resources, p.Names = ns, []string{tg.t.Kind}, []string{name}
	switch tg.t.Kind {
	case simulator.KindPod, simulator.KindDeployment, simulator.KindReplicaSet, simulator.KindService:
	default:
		return Result{}, reject(CodeValidation, "error: cannot expose a %s", tg.t.Kind)
	}
	src := env.State.Get(tg.t.Kind, ns, name)
	if src == nil {
		return Result{}, reject(CodeNotFound, "Error from server (NotFound): %s %q not found", tg.t.display(), name)
	}
	dry, f := dryRun(fl)
	if f != nil {
		return Result{}, f
	}

	selector, ports, f := introspect(src)
	if f != nil && fl.str("selector") == "" {
		return Result{}, f
	}
	if s := fl.str("selector"); s != "" {
		if selector, f = splitLabels(s); f != nil {
			return Result{}, f
		}
	}
	if s := fl.str("port"); s != "" {
		port, f := parsePort(s, "port")
		if f != nil {
			return Result{}, f
		}
		ports = []simulator.ServicePort{{Port: port, TargetPort: simulator.IntOrString{IntVal: port}}}
	}
	if len(ports) == 0 {
		return Result{}, reject(CodeValidation, "error: couldn't find port via --port flag or introspection\nSee 'kubectl expose -h' for help and examples")
	}
	if s := fl.str("target-port"); s != "" {
		target := simulator.IntOrString{IsStr: true, StrVal: s}
		if n, err := strconv.ParseInt(s, 10, 32); err == nil {
			if n < 1 || n > 65535 {
				return Result{}, reject(CodeValidation, "error: invalid argument %q for \"--target-port\" flag: must be a port number or name", s)
			}
			target = simulator.IntOrString{IntVal: int32(n)}
		}
		for i := range ports {
			ports[i].TargetPort = target
		}
	}
	for i := range ports {
		ports[i].Protocol = "TCP"
		if len(ports) > 1 && ports[i].Name == "" {
			ports[i].Name = fmt.Sprintf("port-%d", i+1)
		}
	}
	typ := "ClusterIP"
	switch t := fl.str("type"); strings.ToLower(t) {
	case "", "clusterip":
	case "nodeport":
		typ = "NodePort"
	case "loadbalancer":
		typ = "LoadBalancer"
	case "externalname":
		return Result{}, reject(CodeUnsupported, "error: ExternalName Services aren't simulated; use ClusterIP or NodePort")
	default:
		return Result{}, reject(CodeValidation, "error: invalid --type %q; use ClusterIP, NodePort or LoadBalancer", t)
	}
	svcName := name
	if n := fl.str("name"); n != "" {
		svcName = n
	}
	labels := src.Meta().Labels
	if s := fl.str("labels"); s != "" {
		if labels, f = splitLabels(s); f != nil {
			return Result{}, f
		}
	}
	svc := &simulator.Service{
		Metadata: simulator.ObjectMeta{Name: svcName, Namespace: ns, Labels: copyMap(labels)},
		Spec:     simulator.ServiceSpec{Type: typ, Selector: selector, Ports: ports},
	}
	if len(svc.Metadata.Labels) == 0 {
		svc.Metadata.Labels = nil
	}
	return created(env, svc, "exposed", fl.str("output"), dry)
}

// introspect finds the selector and ports to expose, as kubectl does: a workload's
// matchLabels and its containers' ports, a Pod's labels and ports, or a Service's own.
func introspect(o simulator.Object) (map[string]string, []simulator.ServicePort, *Failure) {
	fromContainers := func(cs []simulator.Container) []simulator.ServicePort {
		var ports []simulator.ServicePort
		for _, c := range cs {
			for _, cp := range c.Ports {
				ports = append(ports, simulator.ServicePort{Name: cp.Name, Port: cp.ContainerPort, TargetPort: simulator.IntOrString{IntVal: cp.ContainerPort}})
			}
		}
		return ports
	}
	workload := func(kind string, sel simulator.LabelSelector, cs []simulator.Container) (map[string]string, []simulator.ServicePort, *Failure) {
		if len(sel.MatchExpressions) > 0 || len(sel.MatchLabels) == 0 {
			return nil, fromContainers(cs), reject(CodeValidation, "error: couldn't retrieve selectors via --selector flag or introspection: the %s's selector uses matchExpressions, which a Service can't use; pass --selector", strings.ToLower(kind))
		}
		return copyMap(sel.MatchLabels), fromContainers(cs), nil
	}
	switch v := o.(type) {
	case *simulator.Deployment:
		return workload(simulator.KindDeployment, v.Spec.Selector, v.Spec.Template.Spec.Containers)
	case *simulator.ReplicaSet:
		return workload(simulator.KindReplicaSet, v.Spec.Selector, v.Spec.Template.Spec.Containers)
	case *simulator.Pod:
		if len(v.Metadata.Labels) == 0 {
			return nil, fromContainers(v.Spec.Containers), reject(CodeValidation, "error: couldn't retrieve selectors via --selector flag or introspection: the pod has no labels. Add one with kubectl label pod %s app=%s, or pass --selector", v.Metadata.Name, v.Metadata.Name)
		}
		return copyMap(v.Metadata.Labels), fromContainers(v.Spec.Containers), nil
	case *simulator.Service:
		if len(v.Spec.Selector) == 0 {
			return nil, nil, reject(CodeValidation, "error: couldn't retrieve selectors via --selector flag or introspection: the service has no selector")
		}
		ports := make([]simulator.ServicePort, 0, len(v.Spec.Ports))
		for _, sp := range v.Spec.Ports {
			ports = append(ports, simulator.ServicePort{Name: sp.Name, Port: sp.Port, TargetPort: sp.TargetPort})
		}
		return copyMap(v.Spec.Selector), ports, nil
	}
	return nil, nil, reject(CodeValidation, "error: cannot expose a %s", o.GetKind())
}
