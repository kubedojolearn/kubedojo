package simulator

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"
)

// ---- helpers ----

func loadYAMLFixture(t *testing.T, path string) Fixture {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := yaml.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	j, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var f Fixture
	if err := json.Unmarshal(j, &f); err != nil {
		t.Fatal(err)
	}
	return f
}

const nodesJSON = `
  {"apiVersion":"v1","kind":"Node","metadata":{"name":"cp","labels":{"node-role.kubernetes.io/control-plane":""}},
   "spec":{"taints":[{"key":"node-role.kubernetes.io/control-plane","effect":"NoSchedule"}]},
   "status":{"capacity":{"cpu":"2","memory":"4Gi","pods":"110"},"allocatable":{"cpu":"1800m","memory":"3Gi","pods":"110"},"conditions":[{"type":"Ready","status":"True"}]},"ageSeconds":864000},
  {"apiVersion":"v1","kind":"Node","metadata":{"name":"worker-1"},
   "status":{"capacity":{"cpu":"4","memory":"8Gi","pods":"110"},"allocatable":{"cpu":"1","memory":"7Gi","pods":"110"},"conditions":[{"type":"Ready","status":"True"}]},"ageSeconds":864000},
  {"apiVersion":"v1","kind":"Node","metadata":{"name":"worker-2"},
   "status":{"capacity":{"cpu":"4","memory":"8Gi","pods":"110"},"allocatable":{"cpu":"1","memory":"7Gi","pods":"110"},"conditions":[{"type":"Ready","status":"True"}]},"ageSeconds":864000}`

func fixture(t *testing.T, resources string, catalog ...ImageProfile) Fixture {
	t.Helper()
	var f Fixture
	raw := `{"schemaVersion":1,"simulatorVersion":"` + Version + `","resources":[` + nodesJSON
	if strings.TrimSpace(resources) != "" {
		raw += "," + resources
	}
	raw += "]}"
	if err := json.Unmarshal([]byte(raw), &f); err != nil {
		t.Fatalf("fixture JSON: %v", err)
	}
	f.ImageCatalog = catalog
	return f
}

func mustLoad(t *testing.T, f Fixture, seed uint64) *State {
	t.Helper()
	s, err := Load(f, seed)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return s
}

func reconcile(t *testing.T, s *State) {
	t.Helper()
	if _, err := Reconcile(s); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
}

func pods(s *State, ns string) []*Pod {
	var out []*Pod
	for _, o := range s.List(KindPod, ns) {
		out = append(out, o.(*Pod))
	}
	return out
}

func deployment(s *State, name string) *Deployment {
	return s.Get(KindDeployment, "default", name).(*Deployment)
}

func eventReasons(s *State) string {
	var out []string
	for _, e := range s.Events() {
		out = append(out, e.Reason)
	}
	return strings.Join(out, ",")
}

const nginxDeployment = `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"nginx","namespace":"default","labels":{"app":"nginx"}},
  "spec":{"replicas":1,"selector":{"matchLabels":{"app":"nginx"}},
  "template":{"metadata":{"labels":{"app":"nginx"}},"spec":{"containers":[{"name":"nginx","image":"nginx:1.27","ports":[{"containerPort":80}],
  "resources":{"requests":{"cpu":"100m","memory":"64Mi"}}}]}}},"ageSeconds":3600}`

// ---- repository fixtures ----

// brokenOnPurpose lists the fixtures whose labs start with failing Pods, and the failures their
// lessons ask learners to diagnose. Every failing Pod must show one of them, and each must occur.
var brokenOnPurpose = map[string][]string{
	"batch-requests.yaml":        {"Unschedulable"},
	"etl-events.yaml":            {"Unschedulable"},
	"pending-reports.yaml":       {"Unschedulable"},
	"render-memory.yaml":         {"Unschedulable"},
	"frontend-bad-image.yaml":    {"ImagePullBackOff"},
	"orders-crash.yaml":          {"CrashLoopBackOff"},
	"worker-missing-config.yaml": {"CreateContainerConfigError"},
	"shop-outage.yaml":           {"ImagePullBackOff", "CreateContainerConfigError"},
}

// podProblem names why a Pod isn't running and ready, or returns "" if it is.
func podProblem(p *Pod) string {
	if p.Spec.NodeName == "" {
		return "Unschedulable"
	}
	for _, cs := range p.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			return cs.State.Waiting.Reason
		}
	}
	if p.Status.PodIP == "" || !p.Ready() {
		return "NotReady"
	}
	return ""
}

func TestEveryRepositoryFixtureLoads(t *testing.T) {
	files, err := filepath.Glob("../../../../content/fixtures/*.yaml")
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures found: %v", err)
	}
	for _, f := range files {
		name := filepath.Base(f)
		t.Run(name, func(t *testing.T) {
			s := mustLoad(t, loadYAMLFixture(t, f), 42)
			expected := brokenOnPurpose[name]
			seen := map[string]bool{}
			for _, p := range pods(s, "*") {
				problem := podProblem(p)
				if problem == "" {
					continue
				}
				seen[problem] = true
				if !slices.Contains(expected, problem) {
					t.Errorf("pod %s/%s not running: %s (node=%q ip=%q phase=%s)", p.Metadata.Namespace, p.Metadata.Name, problem, p.Spec.NodeName, p.Status.PodIP, p.Status.Phase)
				}
			}
			for _, want := range expected {
				if !seen[want] {
					t.Errorf("the lesson expects a Pod with %s, but none has it", want)
				}
			}
		})
	}
}

func TestNginxFixtureMatchesKubernetes(t *testing.T) {
	s := mustLoad(t, loadYAMLFixture(t, "../../../../content/fixtures/nginx-one-replica.yaml"), 7)
	d := deployment(s, "nginx")
	if d.Status.ReadyReplicas != 1 || d.Status.UpdatedReplicas != 1 || d.Status.AvailableReplicas != 1 || d.Metadata.Annotations[RevisionAnnotation] != "1" {
		t.Fatalf("deployment status %+v annotations %v", d.Status, d.Metadata.Annotations)
	}
	rss := s.List(KindReplicaSet, "default")
	if len(rss) != 1 || !strings.HasPrefix(rss[0].Meta().Name, "nginx-") || len(rss[0].Meta().Name) != len("nginx-")+10 {
		t.Fatalf("replica sets %v", rss)
	}
	ps := pods(s, "default")
	if len(ps) != 1 {
		t.Fatalf("pods %d", len(ps))
	}
	p := ps[0]
	if p.Spec.NodeName == "node-1" {
		t.Fatal("pod scheduled onto the tainted control-plane node")
	}
	if !strings.HasPrefix(p.Status.PodIP, "10.244.") || p.Status.HostIP == "" || p.Status.Phase != "Running" {
		t.Fatalf("pod status %+v", p.Status)
	}
	// History replay: the Pod is as old as its Deployment, not created at lab start.
	if !p.Metadata.CreationTimestamp.Equal(d.Metadata.CreationTimestamp) {
		t.Fatalf("pod created %v, deployment %v", p.Metadata.CreationTimestamp, d.Metadata.CreationTimestamp)
	}
	if !s.Has(KindService, "default", "kubernetes") {
		t.Fatal("default kubernetes Service missing")
	}
	for _, ns := range SystemNamespaces {
		if !s.Has(KindNamespace, "", ns) {
			t.Fatalf("namespace %s missing", ns)
		}
	}
	reasons := eventReasons(s)
	for _, want := range []string{"ScalingReplicaSet", "SuccessfulCreate", "Scheduled", "Pulled", "Created", "Started"} {
		if !strings.Contains(reasons, want) {
			t.Errorf("missing event %s in %s", want, reasons)
		}
	}
}

// ---- determinism ----

func snapshot(t *testing.T, s *State) string {
	t.Helper()
	b, err := json.Marshal(struct {
		Objects []Object
		Events  []Event
	}{s.All(), s.Events()})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestDeterminism(t *testing.T) {
	f := fixture(t, nginxDeployment)
	run := func(seed uint64) string {
		s := mustLoad(t, f, seed)
		d := deployment(s, "nginx")
		three := int32(3)
		d.Spec.Replicas = &three
		s.Advance(10 * time.Second)
		if err := s.Put(d); err != nil {
			t.Fatal(err)
		}
		reconcile(t, s)
		return snapshot(t, s)
	}
	a, b := run(99), run(99)
	if a != b {
		t.Fatal("identical fixture, seed and input produced different state")
	}
	if c := run(100); c == a {
		t.Fatal("different seeds should produce different generated names")
	}
}

func TestReconcileIsIdempotent(t *testing.T) {
	s := mustLoad(t, fixture(t, nginxDeployment), 1)
	rev := s.Revision
	res, err := Reconcile(s)
	if err != nil || res.Passes != 1 || s.Revision != rev {
		t.Fatalf("stable state changed: passes=%d err=%v", res.Passes, err)
	}
}

// ---- workloads ----

func scaleTo(t *testing.T, s *State, name string, n int32) {
	t.Helper()
	d := deployment(s, name)
	d.Spec.Replicas = &n
	s.Advance(10 * time.Second)
	if err := s.Put(d); err != nil {
		t.Fatal(err)
	}
	reconcile(t, s)
}

func TestScaling(t *testing.T) {
	s := mustLoad(t, fixture(t, nginxDeployment), 1)
	scaleTo(t, s, "nginx", 3)
	d := deployment(s, "nginx")
	if d.Status.ReadyReplicas != 3 || len(pods(s, "default")) != 3 {
		t.Fatalf("after scale up: %+v, %d pods", d.Status, len(pods(s, "default")))
	}
	if d.Metadata.Generation != 2 || d.Status.ObservedGeneration != 2 {
		t.Fatalf("generation %d observed %d", d.Metadata.Generation, d.Status.ObservedGeneration)
	}
	if !strings.Contains(eventMessages(s), "Scaled up replica set nginx-") || !strings.Contains(eventMessages(s), "from 1 to 3") {
		t.Fatalf("events: %s", eventMessages(s))
	}
	// Two workers with 1 CPU each: Pods spread by lowest requested fraction.
	nodes := map[string]int{}
	for _, p := range pods(s, "default") {
		nodes[p.Spec.NodeName]++
	}
	if nodes["worker-1"] == 0 || nodes["worker-2"] == 0 {
		t.Fatalf("pods not spread: %v", nodes)
	}

	scaleTo(t, s, "nginx", 0)
	if len(pods(s, "default")) != 0 || deployment(s, "nginx").Status.ReadyReplicas != 0 {
		t.Fatal("scale to zero left pods")
	}
}

func eventMessages(s *State) string {
	var out []string
	for _, e := range s.Events() {
		out = append(out, e.Reason+": "+e.Message)
	}
	return strings.Join(out, "\n")
}

func setImage(t *testing.T, s *State, image string) {
	t.Helper()
	d := deployment(s, "nginx")
	d.Spec.Template.Spec.Containers[0].Image = image
	s.Advance(10 * time.Second)
	if err := s.Put(d); err != nil {
		t.Fatal(err)
	}
	reconcile(t, s)
}

func TestRollingUpdateAndRollback(t *testing.T) {
	s := mustLoad(t, fixture(t, nginxDeployment), 1)
	scaleTo(t, s, "nginx", 4)
	oldHash := s.List(KindReplicaSet, "default")[0].Meta().Labels[PodTemplateHashLabel]

	setImage(t, s, "nginx:1.26")
	d := deployment(s, "nginx")
	if d.Metadata.Annotations[RevisionAnnotation] != "2" || d.Status.UpdatedReplicas != 4 || d.Status.ReadyReplicas != 4 {
		t.Fatalf("after rollout: %+v %v", d.Status, d.Metadata.Annotations)
	}
	rss := s.List(KindReplicaSet, "default")
	if len(rss) != 2 {
		t.Fatalf("expected old and new ReplicaSets, got %d", len(rss))
	}
	for _, p := range pods(s, "default") {
		if p.Spec.Containers[0].Image != "nginx:1.26" {
			t.Fatalf("old pod survived rollout: %s", p.Metadata.Name)
		}
	}
	// The rollout happened in steps: several scale events, never more than desired+maxSurge Pods.
	if n := strings.Count(eventMessages(s), "ScalingReplicaSet"); n < 3 {
		t.Fatalf("rollout was not stepwise:\n%s", eventMessages(s))
	}

	// Rollback: the old template's ReplicaSet becomes revision 3.
	setImage(t, s, "nginx:1.27")
	d = deployment(s, "nginx")
	if d.Metadata.Annotations[RevisionAnnotation] != "3" {
		t.Fatalf("rollback revision %v", d.Metadata.Annotations)
	}
	for _, o := range s.List(KindReplicaSet, "default") {
		rs := o.(*ReplicaSet)
		if rs.Metadata.Labels[PodTemplateHashLabel] == oldHash && (rs.Metadata.Annotations[RevisionAnnotation] != "3" || rs.DesiredReplicas() != 4) {
			t.Fatalf("rolled-back ReplicaSet %+v", rs.Metadata.Annotations)
		}
	}
	if len(s.List(KindReplicaSet, "default")) != 2 {
		t.Fatal("rollback must reuse the old ReplicaSet, not create a third")
	}
}

func TestRecreateStrategy(t *testing.T) {
	dep := strings.Replace(nginxDeployment, `"spec":{"replicas":1,`, `"spec":{"replicas":2,"strategy":{"type":"Recreate"},`, 1)
	s := mustLoad(t, fixture(t, dep), 1)
	setImage(t, s, "nginx:1.26")
	msgs := eventMessages(s)
	down := strings.Index(msgs, "Scaled down replica set")
	up := strings.LastIndex(msgs, "Scaled up replica set")
	if down < 0 || up < down {
		t.Fatalf("Recreate must scale down before scaling up:\n%s", msgs)
	}
	if deployment(s, "nginx").Status.ReadyReplicas != 2 {
		t.Fatal("recreate did not finish")
	}
}

// ---- scheduling ----

func TestSchedulingFailuresExplainThemselves(t *testing.T) {
	dep := strings.Replace(nginxDeployment, `"spec":{"containers"`, `"spec":{"nodeSelector":{"disktype":"ssd"},"containers"`, 1)
	s := mustLoad(t, fixture(t, dep), 1)
	p := pods(s, "default")[0]
	if p.Spec.NodeName != "" || p.Status.Phase != "Pending" {
		t.Fatalf("pod should be pending: %+v", p.Status)
	}
	want := "0/3 nodes are available: 1 node(s) had untolerated taint {node-role.kubernetes.io/control-plane: }, 2 node(s) didn't match Pod's node affinity/selector."
	if !strings.Contains(eventMessages(s), "FailedScheduling: "+want) {
		t.Fatalf("missing FailedScheduling %q in\n%s", want, eventMessages(s))
	}
	if deployment(s, "nginx").Status.ReadyReplicas != 0 {
		t.Fatal("deployment reports a ready replica")
	}

	// Labelling a node fixes it on the next reconcile.
	n := s.Get(KindNode, "", "worker-2").(*Node)
	n.Metadata.Labels["disktype"] = "ssd"
	if err := s.Put(n); err != nil {
		t.Fatal(err)
	}
	reconcile(t, s)
	if p := pods(s, "default")[0]; p.Spec.NodeName != "worker-2" || !p.Ready() {
		t.Fatalf("pod not scheduled to labelled node: %q", p.Spec.NodeName)
	}
}

func TestInsufficientCPU(t *testing.T) {
	dep := strings.Replace(nginxDeployment, `"cpu":"100m"`, `"cpu":"600m"`, 1)
	s := mustLoad(t, fixture(t, dep), 1)
	scaleTo(t, s, "nginx", 3) // each worker fits one 600m Pod
	running := 0
	for _, p := range pods(s, "default") {
		if p.Spec.NodeName != "" {
			running++
		}
	}
	if running != 2 || deployment(s, "nginx").Status.ReadyReplicas != 2 {
		t.Fatalf("expected 2 of 3 scheduled, got %d", running)
	}
	if !strings.Contains(eventMessages(s), "2 Insufficient cpu") {
		t.Fatalf("events:\n%s", eventMessages(s))
	}
}

// ---- kubelet ----

func TestImagePullFailure(t *testing.T) {
	dep := strings.Replace(nginxDeployment, `"image":"nginx:1.27"`, `"image":"ngnix:1.27"`, 1)
	s := mustLoad(t, fixture(t, dep), 1)
	p := pods(s, "default")[0]
	cs := p.Status.ContainerStatuses[0]
	if cs.State.Waiting == nil || cs.State.Waiting.Reason != "ImagePullBackOff" || p.Status.Phase != "Pending" || p.Status.PodIP == "" {
		t.Fatalf("status %+v", p.Status)
	}
	if !strings.Contains(eventMessages(s), `Failed: Failed to pull image "ngnix:1.27"`) {
		t.Fatalf("events:\n%s", eventMessages(s))
	}
	setImage(t, s, "nginx:1.27")
	if deployment(s, "nginx").Status.ReadyReplicas != 1 {
		t.Fatal("fixing the image did not recover")
	}
}

func TestMissingConfigMapThenCreated(t *testing.T) {
	dep := strings.Replace(nginxDeployment, `"ports":[{"containerPort":80}]`,
		`"ports":[{"containerPort":80}],"env":[{"name":"MODE","valueFrom":{"configMapKeyRef":{"name":"app-config","key":"mode"}}}]`, 1)
	s := mustLoad(t, fixture(t, dep), 1)
	cs := pods(s, "default")[0].Status.ContainerStatuses[0]
	if cs.State.Waiting == nil || cs.State.Waiting.Reason != "CreateContainerConfigError" || cs.State.Waiting.Message != `configmap "app-config" not found` {
		t.Fatalf("status %+v", cs)
	}
	cm := &ConfigMap{Metadata: ObjectMeta{Name: "app-config", Namespace: "default"}, Data: map[string]string{"other": "x"}}
	if err := s.Put(cm); err != nil {
		t.Fatal(err)
	}
	reconcile(t, s)
	cs = pods(s, "default")[0].Status.ContainerStatuses[0]
	if cs.State.Waiting == nil || cs.State.Waiting.Message != "couldn't find key mode in ConfigMap default/app-config" {
		t.Fatalf("missing key: %+v", cs)
	}
	cm = s.Get(KindConfigMap, "default", "app-config").(*ConfigMap)
	cm.Data["mode"] = "prod"
	if err := s.Put(cm); err != nil {
		t.Fatal(err)
	}
	reconcile(t, s)
	if !pods(s, "default")[0].Ready() {
		t.Fatal("pod not running after ConfigMap fixed")
	}
}

func TestCrashLoopFollowsTheLogicalClock(t *testing.T) {
	crasher := ImageProfile{Image: "kubedojo/orders:1.0", Behavior: "crash", ExitCode: 1, RequiresEnv: []string{"APP_MODE"}, Logs: []string{"fatal: APP_MODE is not set"}}
	dep := strings.Replace(nginxDeployment, `"image":"nginx:1.27"`, `"image":"kubedojo/orders:1.0"`, 1)
	dep = strings.Replace(dep, `"ageSeconds":3600`, `"ageSeconds":0`, 1)
	s := mustLoad(t, fixture(t, dep, crasher), 1)
	cs := pods(s, "default")[0].Status.ContainerStatuses[0]
	if cs.State.Waiting == nil || cs.State.Waiting.Reason != "CrashLoopBackOff" || cs.LastState.Terminated == nil || cs.LastState.Terminated.ExitCode != 1 {
		t.Fatalf("status %+v", cs)
	}
	if cs.RestartCount != 0 {
		t.Fatalf("restarts at t=0: %d", cs.RestartCount)
	}
	// Reads don't move time. A mutation tick does; restarts follow 10s, 20s, 40s… backoff.
	s.Advance(75 * time.Second)
	reconcile(t, s)
	if got := pods(s, "default")[0].Status.ContainerStatuses[0].RestartCount; got != 3 {
		t.Fatalf("restarts after 75s: %d (want 3: 10+20+40)", got)
	}
	if pods(s, "default")[0].Status.Phase != "Running" {
		t.Fatal("crash-looping pods are in phase Running, like Kubernetes")
	}
	// Providing the required variable fixes it.
	d := deployment(s, "nginx")
	d.Spec.Template.Spec.Containers[0].Env = []EnvVar{{Name: "APP_MODE", Value: "prod"}}
	s.Advance(10 * time.Second)
	if err := s.Put(d); err != nil {
		t.Fatal(err)
	}
	reconcile(t, s)
	if deployment(s, "nginx").Status.ReadyReplicas != 1 {
		t.Fatal("fixed container still crashing")
	}
}

func TestCrashBackoffSchedule(t *testing.T) {
	cases := map[time.Duration]int32{0: 0, 9 * time.Second: 0, 10 * time.Second: 1, 30 * time.Second: 2, 70 * time.Second: 3, 10 * time.Minute: 5}
	for elapsed, want := range cases {
		if got, _, _ := crashBackoff(elapsed); got != want {
			t.Errorf("%v: %d, want %d", elapsed, got, want)
		}
	}
}

// ---- services ----

const webService = `{"apiVersion":"v1","kind":"Service","metadata":{"name":"web","namespace":"default"},
  "spec":{"selector":{"app":"nginx"},"ports":[{"port":80,"targetPort":80}]}}`

func TestServicesAndEndpointSlices(t *testing.T) {
	s := mustLoad(t, fixture(t, nginxDeployment+","+webService), 1)
	scaleTo(t, s, "nginx", 3)
	svc := s.Get(KindService, "default", "web").(*Service)
	if svc.Spec.ClusterIP != "10.96.0.10" || svc.Spec.Type != "ClusterIP" {
		t.Fatalf("service %+v", svc.Spec)
	}
	slices := s.List(KindEndpointSlice, "default")
	if len(slices) != 1 {
		t.Fatalf("slices %d", len(slices))
	}
	slice := slices[0].(*EndpointSlice)
	if slice.Metadata.Labels[serviceNameLabel] != "web" || len(slice.Endpoints) != 3 || slice.Ports[0].Port != 80 {
		t.Fatalf("slice %+v", slice)
	}
	for _, e := range slice.Endpoints {
		if !e.Conditions.Ready || e.TargetRef == nil {
			t.Fatalf("endpoint %+v", e)
		}
	}

	// Selector mismatch empties the slice (debug-missing-endpoints).
	svc.Spec.Selector = map[string]string{"app": "web-app"}
	if err := s.Put(svc); err != nil {
		t.Fatal(err)
	}
	reconcile(t, s)
	if n := len(s.List(KindEndpointSlice, "default")[0].(*EndpointSlice).Endpoints); n != 0 {
		t.Fatalf("expected no endpoints, got %d", n)
	}

	// NodePort allocation is deterministic and in range.
	svc = s.Get(KindService, "default", "web").(*Service)
	svc.Spec.Type = "NodePort"
	if err := s.Put(svc); err != nil {
		t.Fatal(err)
	}
	reconcile(t, s)
	np := s.Get(KindService, "default", "web").(*Service).Spec.Ports[0].NodePort
	if np < 30000 || np > 32767 {
		t.Fatalf("nodePort %d", np)
	}
}

// ---- garbage collection and namespaces ----

func TestDeletingAnOwnerCascades(t *testing.T) {
	s := mustLoad(t, fixture(t, nginxDeployment+","+webService), 1)
	s.Delete(KindDeployment, "default", "nginx")
	reconcile(t, s)
	if len(s.List(KindReplicaSet, "default")) != 0 || len(pods(s, "default")) != 0 {
		t.Fatal("dependents survived owner deletion")
	}
	s.Delete(KindService, "default", "web")
	reconcile(t, s)
	if len(s.List(KindEndpointSlice, "default")) != 0 {
		t.Fatal("slice survived service deletion")
	}
}

func TestDeletingANamespaceDeletesItsObjects(t *testing.T) {
	ns := `{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"shop"}}`
	cm := `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"cfg","namespace":"shop"},"data":{"a":"b"}}`
	s := mustLoad(t, fixture(t, ns+","+cm), 1)
	s.Delete(KindNamespace, "", "shop")
	reconcile(t, s)
	if s.Has(KindConfigMap, "shop", "cfg") {
		t.Fatal("namespace deletion left objects behind")
	}
}

// ---- validation and input ----

func TestValidationAndUnsupportedInput(t *testing.T) {
	bad := map[string]string{
		"uppercase name":      `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"Bad","namespace":"default"}}`,
		"selector mismatch":   `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"d","namespace":"default"},"spec":{"selector":{"matchLabels":{"app":"a"}},"template":{"metadata":{"labels":{"app":"b"}},"spec":{"containers":[{"name":"c","image":"nginx"}]}}}}`,
		"no containers":       `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p","namespace":"default"},"spec":{"containers":[]}}`,
		"bad label value":     `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p","namespace":"default","labels":{"a":"-x-"}},"spec":{"containers":[{"name":"c","image":"nginx"}]}}`,
		"too many replicas":   `{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"d","namespace":"default"},"spec":{"replicas":500,"selector":{"matchLabels":{"app":"a"}},"template":{"metadata":{"labels":{"app":"a"}},"spec":{"containers":[{"name":"c","image":"nginx"}]}}}}`,
		"gpu request":         `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p","namespace":"default"},"spec":{"containers":[{"name":"c","image":"nginx","resources":{"requests":{"nvidia.com/gpu":"1"}}}]}}`,
		"bad nodePort":        `{"apiVersion":"v1","kind":"Service","metadata":{"name":"s","namespace":"default"},"spec":{"type":"NodePort","ports":[{"port":80,"nodePort":80}]}}`,
		"requests over limit": `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p","namespace":"default"},"spec":{"containers":[{"name":"c","image":"nginx","resources":{"requests":{"cpu":"2"},"limits":{"cpu":"1"}}}]}}`,
	}
	for name, raw := range bad {
		o, err := DecodeInput([]byte(raw))
		if err != nil {
			continue // rejected at decode is fine too
		}
		var ve *ValidationError
		if err := Validate(o); !errors.As(err, &ve) {
			t.Errorf("%s: expected a validation error, got %v", name, err)
		}
	}

	unsupported := map[string]string{
		"StatefulSet":      `{"apiVersion":"apps/v1","kind":"StatefulSet","metadata":{"name":"db"}}`,
		"wrong apiVersion": `{"apiVersion":"extensions/v1beta1","kind":"Deployment","metadata":{"name":"d"}}`,
		"init containers":  `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p","namespace":"default"},"spec":{"initContainers":[],"containers":[{"name":"c","image":"nginx"}]}}`,
		"volumes":          `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p","namespace":"default"},"spec":{"volumes":[],"containers":[{"name":"c","image":"nginx"}]}}`,
		"probes":           `{"apiVersion":"v1","kind":"Pod","metadata":{"name":"p","namespace":"default"},"spec":{"containers":[{"name":"c","image":"nginx","livenessProbe":{}}]}}`,
	}
	for name, raw := range unsupported {
		_, err := DecodeInput([]byte(raw))
		var ue *UnsupportedError
		if !errors.As(err, &ue) {
			t.Errorf("%s: expected UnsupportedError, got %v", name, err)
		}
	}

	sec, err := DecodeInput([]byte(`{"apiVersion":"v1","kind":"Secret","metadata":{"name":"s","namespace":"default"},"stringData":{"user":"admin"}}`))
	if err != nil || string(sec.(*Secret).Data["user"]) != "admin" || sec.(*Secret).Type != "Opaque" {
		t.Fatalf("stringData: %v %+v", err, sec)
	}
}

func TestImmutableFields(t *testing.T) {
	s := mustLoad(t, fixture(t, nginxDeployment), 1)
	old := deployment(s, "nginx")
	changed := old.DeepCopy().(*Deployment)
	changed.Spec.Selector.MatchLabels = map[string]string{"app": "other"}
	if err := ValidateUpdate(old, changed); err == nil {
		t.Fatal("deployment selector must be immutable")
	}
	p := pods(s, "default")[0]
	p2 := p.DeepCopy().(*Pod)
	p2.Spec.Containers[0].Image = "nginx:1.26"
	if err := ValidateUpdate(p, p2); err == nil {
		t.Fatal("pod containers must be immutable")
	}
	p3 := p.DeepCopy().(*Pod)
	p3.Metadata.Labels["tier"] = "web"
	if err := ValidateUpdate(p, p3); err != nil {
		t.Fatalf("pod labels are mutable: %v", err)
	}
}

func TestObjectLimit(t *testing.T) {
	s := mustLoad(t, fixture(t, ""), 1)
	s.Limits.MaxObjects = s.Count() + 2
	for i := 0; i < 2; i++ {
		cm := &ConfigMap{Metadata: ObjectMeta{Name: "cm-" + string(rune('a'+i)), Namespace: "default"}}
		if err := s.Put(cm); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Put(&ConfigMap{Metadata: ObjectMeta{Name: "one-too-many", Namespace: "default"}}); !errors.Is(err, ErrLimit) {
		t.Fatalf("expected ErrLimit, got %v", err)
	}
}

// ---- storage ----

func TestEncodeDecodeRoundTrip(t *testing.T) {
	s := mustLoad(t, fixture(t, nginxDeployment+","+webService+`,{"apiVersion":"v1","kind":"Secret","metadata":{"name":"creds","namespace":"default"},"stringData":{"password":"training-only"}}`), 3)
	restored := NewState(s.Now, s.Gen.Seed, s.Images)
	var objs []Object
	for _, o := range s.All() {
		st, err := Encode(o)
		if err != nil {
			t.Fatalf("encode %s: %v", KeyOf(o), err)
		}
		back, err := DecodeStored(st)
		if err != nil {
			t.Fatalf("decode %s: %v", KeyOf(o), err)
		}
		objs = append(objs, back)
	}
	if err := restored.Restore(objs); err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(s.All())
	b, _ := json.Marshal(restored.All())
	if string(a) != string(b) {
		t.Fatal("state changed through storage round trip")
	}
	if restored.Revision != s.Revision {
		t.Fatalf("revision %d vs %d", restored.Revision, s.Revision)
	}
	// Reconciling restored state changes nothing.
	rev := restored.Revision
	reconcile(t, restored)
	if restored.Revision != rev {
		t.Fatal("restored state was not stable")
	}
}

func TestChangesTrackWritesAndDeletes(t *testing.T) {
	s := mustLoad(t, fixture(t, nginxDeployment), 1)
	restored := NewState(s.Now, s.Gen.Seed, s.Images)
	if err := restored.Restore(s.All()); err != nil {
		t.Fatal(err)
	}
	restored.Gen = s.Gen
	scaleTo(t, restored, "nginx", 2)
	up, del := restored.Changes()
	if len(del) != 0 || len(up) < 3 {
		t.Fatalf("changes up=%d del=%d", len(up), len(del))
	}
	restored.Delete(KindDeployment, "default", "nginx")
	reconcile(t, restored)
	_, del = restored.Changes()
	if len(del) < 4 { // deployment, replica set, two pods
		t.Fatalf("deletes %v", del)
	}
}

func TestRebaseKeepsResourceVersionsIncreasing(t *testing.T) {
	s := mustLoad(t, fixture(t, nginxDeployment), 1)
	before := s.Get(KindDeployment, "default", "nginx").Meta().ResourceVersion
	rev := s.Revision
	s.Rebase(1000)
	after := s.Get(KindDeployment, "default", "nginx").Meta().ResourceVersion
	if s.Revision != rev+1000 || after == before {
		t.Fatalf("revision %d->%d, rv %s->%s", rev, s.Revision, before, after)
	}
	scaleTo(t, s, "nginx", 2)
	if got := deployment(s, "nginx").Metadata.ResourceVersion; got <= after && len(got) <= len(after) {
		t.Fatalf("new write got rv %s, not above %s", got, after)
	}
}
