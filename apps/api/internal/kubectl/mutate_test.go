package kubectl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// scenarios are command sequences run in order against a fresh lab. Each produces one golden
// transcript in testdata/scenarios, so a reviewer can read what a learner would see.
var scenarios = map[string][]string{
	"scale": {
		"kubectl scale deploy nginx --replicas=3",
		"kubectl get deploy nginx",
		"kubectl get rs",
		"kubectl get pods -o wide",
		"kubectl events --for deployment/nginx",
		"kubectl scale deployment/nginx --replicas=1 --current-replicas=2",
		"kubectl scale deploy nginx --replicas=1 --current-replicas=3",
		"kubectl get pods",
		"kubectl scale deploy nginx --replicas=51",
		"kubectl scale deploy nginx --replicas=-1",
		"kubectl scale deploy nginx",
		"kubectl scale svc web --replicas=2",
		"kubectl scale deploy nope --replicas=2",
		"kubectl scale deploy --all -n shop --replicas=0",
		"kubectl get deploy -n shop",
	},
	"run": {
		"kubectl run web --image=nginx:1.27 --port=80 --env=MODE=prod --labels=app=web,tier=frontend",
		"kubectl get pods -o wide --show-labels",
		"kubectl describe pod web",
		"kubectl run web --image=nginx:1.27",
		"kubectl run Web --image=nginx:1.27",
		"kubectl run broken --image=ngnix:1.27",
		"kubectl get pod broken",
		"kubectl events --for pod/broken",
		"kubectl run x --image=nginx:1.27 --restart=Never",
		"kubectl run x -it --rm --image=busybox",
		"kubectl run x --image=busybox -- sleep 3600",
		"kubectl run x",
		"kubectl run x --image=nginx:1.27 -n nope",
		"kubectl run draft --image=nginx:1.27 --port=80 --dry-run=client -o yaml",
		"kubectl run draft --image=nginx:1.27 --dry-run=client",
		"kubectl run draft --image=nginx:1.27 --dry-run=server",
		"kubectl get pod draft",
	},
	"create": {
		"kubectl create deployment api --image=kubedojo/demo-api:1.0 --replicas=2 --port=8080",
		"kubectl get deploy,rs,pods -l app=api",
		"kubectl create deploy api --image=nginx:1.27",
		"kubectl create deployment two --image=nginx:1.27 --image=redis:7 --dry-run=client -o yaml",
		"kubectl create deployment big --image=nginx:1.27 --replicas=60",
		"kubectl create deployment noimage",
		"kubectl create namespace dev",
		"kubectl create ns Dev",
		"kubectl create configmap settings --from-literal=mode=prod --from-literal=level=info -n dev",
		"kubectl create cm dup --from-literal=a=1 --from-literal=a=2",
		"kubectl create cm bad --from-literal=nokey",
		"kubectl get cm settings -n dev -o yaml",
		"kubectl create secret generic db --from-literal=password=practice-only",
		"kubectl describe secret db",
		"kubectl create secret tls cert",
		"kubectl create secret db2",
		"kubectl create service nodeport api --tcp=80:8080 --node-port=30080",
		"kubectl create service clusterip cache --tcp=6379",
		"kubectl get svc -o wide",
		"kubectl get endpointslices",
		"kubectl create service externalname ext --tcp=80",
		"kubectl create service clusterip nop",
		"kubectl create job nightly --image=busybox",
		"kubectl create pod x",
		"kubectl create",
		"kubectl create -f manifests/web.yaml",
		"kubectl get ns",
	},
	"expose": {
		"kubectl expose deployment nginx --port=80 --name=nginx-svc",
		"kubectl get svc nginx-svc -o wide",
		"kubectl get endpoints nginx-svc",
		"kubectl describe svc nginx-svc",
		"kubectl expose deployment nginx --port=80 --name=nginx-svc",
		"kubectl expose deploy nginx --type=NodePort --target-port=80 --name=nginx-np",
		"kubectl get svc nginx-np",
		"kubectl expose pod typo",
		"kubectl expose pod typo --port=8080",
		"kubectl expose svc web --name=web-copy --port=8080 --target-port=http",
		"kubectl expose ns default",
		"kubectl expose deploy nope --port=80",
		"kubectl expose deploy nginx --port=80 --name=lb --type=LoadBalancer --dry-run=client -o yaml",
		"kubectl expose deploy nginx --port=80 --name=lb --type=LoadBalancer",
		"kubectl get svc lb",
	},
	"delete": {
		"kubectl delete pod typo",
		"kubectl delete pod typo",
		"kubectl delete pod typo --ignore-not-found",
		"kubectl delete pods -l app=nginx",
		"kubectl get pods -l app=nginx",
		"kubectl events --for replicaset/nginx-4fzxcz7snh",
		"kubectl delete pods",
		"kubectl delete pods -l app=nope",
		"kubectl delete pod picky --force --grace-period=0",
		"kubectl delete ns default",
		"kubectl delete node node-1",
		"kubectl delete svc kubernetes",
		"kubectl delete endpointslices --all",
		"kubectl delete events --all",
		"kubectl delete cm,secret --all -o name",
		"kubectl delete deploy nginx --cascade=orphan",
		"kubectl get deploy,rs,pods",
		"kubectl delete rs --all",
		"kubectl get pods",
		"kubectl delete ns shop",
		"kubectl get all -n shop",
		"kubectl get ns",
		"kubectl delete svc web web-np nope",
		"kubectl get svc",
	},
	"rollout": {
		"kubectl rollout history deploy/nginx",
		"kubectl set image deployment/nginx nginx=nginx:1.26",
		"kubectl rollout status deployment/nginx",
		"kubectl rollout history deployment/nginx",
		"kubectl get rs",
		"kubectl set image deploy/nginx nginx=nginx:9.99",
		"kubectl rollout status deploy/nginx",
		"kubectl get pods",
		"kubectl rollout undo deploy/nginx",
		"kubectl rollout status deploy/nginx",
		"kubectl rollout history deploy/nginx",
		"kubectl get deploy nginx -o wide",
		"kubectl rollout undo deploy/nginx --to-revision=3",
		"kubectl rollout undo deploy/nginx --to-revision=9",
		"kubectl rollout restart deploy/nginx",
		"kubectl rollout history deploy/nginx",
		"kubectl get rs",
		"kubectl rollout pause deploy/nginx",
		"kubectl rollout undo svc/web",
		"kubectl set image deploy/nginx nope=nginx:1.27",
		"kubectl set image pod/typo web=nginx:1.27",
		"kubectl set image deploy/nginx",
		"kubectl set resources deploy/nginx --limits=cpu=200m",
	},
	"env": {
		"kubectl get pods -n shop",
		"kubectl set env deploy/orders -n shop --from=configmap/orders-config",
		"kubectl create configmap orders-config -n shop --from-literal=mode=prod",
		"kubectl get pods -n shop",
		"kubectl set env deploy/nginx LEVEL=debug MODE=prod",
		"kubectl describe deploy nginx",
		"kubectl set env deploy/nginx MODE-",
		"kubectl set env deploy/nginx --from=secret/db-creds -c nginx",
		"kubectl get deploy nginx -o yaml",
		"kubectl set env deploy/nginx -c nope X=1",
		"kubectl set env deploy/nginx",
	},
	"labels": {
		"kubectl label pod typo tier=web",
		"kubectl label pod typo run=other",
		"kubectl label pod typo run=other --overwrite",
		"kubectl label pod typo tier=web",
		"kubectl label pod typo tier- nope-",
		"kubectl label pod typo 'Bad Key=x'",
		"kubectl label pod typo",
		"kubectl get pods --show-labels",
		"kubectl label pods -l app=nginx app=debug --overwrite",
		"kubectl get pods --show-labels",
		"kubectl get rs",
		"kubectl describe pod picky",
		"kubectl label node node-2 disktype=ssd",
		"kubectl get pod picky -o wide",
		"kubectl label nodes --all zone=lab",
		"kubectl get nodes --show-labels",
		"kubectl annotate deploy nginx team=platform owner=dojo",
		"kubectl annotate deploy/nginx team=apps",
		"kubectl annotate deploy/nginx team=apps --overwrite",
		"kubectl annotate deploy nginx owner-",
		"kubectl describe deploy nginx",
		"kubectl label endpointslices --all x=y",
		"kubectl label events --all x=y",
	},
	"config": {
		"kubectl config set-context --current --namespace=shop",
		"kubectl config get-contexts",
		"kubectl get pods",
		"kubectl run web --image=nginx:1.27",
		"kubectl get pods -o wide",
		"kubectl config set-context --current --namespace=staging",
		"kubectl get pods",
		"kubectl config set-context --current --namespace=Bad",
		"kubectl config set-context prod --namespace=x",
		"kubectl config set-context --current",
		"kubectl config set-context --current --namespace=default",
		"kubectl get pods",
	},
	"unsupported": {
		"kubectl edit deploy nginx",
		"kubectl patch deploy nginx -p '{}'",
		"kubectl replace -f manifests/web.yaml",
		"kubectl apply -f manifests/web.yaml",
		"kubectl delete -f manifests/web.yaml",
		"kubectl create cronjob x --image=busybox --schedule='* * * * *'",
		"kubectl create deployment x --image=nginx:1.27 --dry-run",
		"kubectl run x --image=nginx:1.27 --overrides='{}'",
	},
}

func scenarioFile(name string) string { return filepath.Join("testdata", "scenarios", name+".txt") }

// runScenario runs commands in order and checks the invariants every command must keep: a
// command that didn't mutate leaves the state exactly as it was (clock included); one that
// did moved the clock by one tick (except kubeconfig changes) and left a reconciled cluster.
func runScenario(t *testing.T, cmds []string) string {
	t.Helper()
	env := testEnv(t)
	var out strings.Builder
	for _, cmd := range cmds {
		before := fingerprintOf(t, env.State)
		r := Execute(cmd, env)
		if r.Err != nil {
			t.Fatalf("%s: internal error: %v", cmd, r.Err)
		}
		out.WriteString(render(cmd, r))
		out.WriteString("\n")
		after := fingerprintOf(t, env.State)
		if !r.Mutated {
			if after != before {
				t.Fatalf("%q changed the cluster but didn't report a mutation", cmd)
			}
			continue
		}
		wantNow := before.now.Add(simulator.Tick)
		if strings.Contains(cmd, "config set-context") {
			wantNow = before.now
		}
		if !after.now.Equal(wantNow) {
			t.Fatalf("%q moved the clock to %s, want %s", cmd, after.now, wantNow)
		}
		if res, err := simulator.Reconcile(env.State.Clone()); err != nil || res.Passes != 1 {
			t.Fatalf("%q left an unreconciled cluster (passes=%d, err=%v)", cmd, res.Passes, err)
		}
	}
	return out.String()
}

func TestMutationScenarios(t *testing.T) {
	if *update {
		_ = os.RemoveAll(filepath.Join("testdata", "scenarios"))
		if err := os.MkdirAll(filepath.Join("testdata", "scenarios"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, cmds := range scenarios {
		t.Run(name, func(t *testing.T) {
			got := runScenario(t, cmds)
			if again := runScenario(t, cmds); again != got {
				t.Fatal("the same commands produced different output in an identical lab")
			}
			path := scenarioFile(name)
			if *update {
				if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%v (run go test ./internal/kubectl -update)", err)
			}
			if got != string(want) {
				t.Errorf("output differs from %s (run go test ./internal/kubectl -update and review the diff)\n--- got\n%s", path, got)
			}
		})
	}
}

func TestFailedMutationLeavesNoTrace(t *testing.T) {
	env := testEnv(t)
	before := fingerprintOf(t, env.State)
	for _, cmd := range []string{
		"kubectl run Web --image=nginx:1.27",
		"kubectl create deployment big --image=nginx:1.27 --replicas=60",
		"kubectl delete ns default",
		"kubectl scale deploy nope --replicas=2",
		"kubectl create deployment x --image=nginx:1.27 --dry-run=client -o yaml",
	} {
		if r := Execute(cmd, env); r.Mutated || r.Err != nil {
			t.Fatalf("%q: mutated=%v err=%v", cmd, r.Mutated, r.Err)
		}
	}
	if fingerprintOf(t, env.State) != before {
		t.Fatal("a failed command changed the lab")
	}
}

func TestLabResourceLimitRefusesWholeCommand(t *testing.T) {
	env := testEnv(t)
	env.State.Limits.MaxObjects = env.State.Count() + 3
	before := fingerprintOf(t, env.State)
	// A Deployment, its ReplicaSet and 5 Pods don't fit in 3 free slots.
	r := Execute("kubectl create deployment big --image=nginx:1.27 --replicas=5", env)
	if r.Mutated || r.ExitCode == 0 || r.ErrorCode != CodeValidation || !strings.Contains(r.Stderr, "limit") {
		t.Fatalf("got %+v", r)
	}
	if fingerprintOf(t, env.State) != before {
		t.Fatal("a refused command left part of its changes behind")
	}
}

func TestSecretValuesAreRedactedInDryRunHistory(t *testing.T) {
	env := testEnv(t)
	r := Execute("kubectl create secret generic s --from-literal=token=practice --dry-run=client -o yaml", env)
	if !strings.Contains(r.Stdout, "cHJhY3RpY2U=") || strings.Contains(r.Redacted, "cHJhY3RpY2U=") {
		t.Fatalf("stdout:\n%s\nredacted:\n%s", r.Stdout, r.Redacted)
	}
}

func TestMutationRecordsParsedTarget(t *testing.T) {
	r := Execute("kubectl -n shop scale deploy orders --replicas=1", testEnv(t))
	p := r.Parsed
	if p == nil || p.Verb != "scale" || p.Namespace != "shop" || len(p.Resources) != 1 || p.Resources[0] != simulator.KindDeployment ||
		len(p.Names) != 1 || p.Names[0] != "orders" {
		t.Fatalf("parsed %+v", p)
	}
}
