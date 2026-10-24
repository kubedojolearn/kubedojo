# V1 course outline — 30 lessons

- **Status:** Accepted (Part 01). Lesson text is written in Parts 07 and 17; this file freezes IDs, scope and grading.
- Lesson IDs are `<course-slug>/<lesson-slug>` and never change once published. Titles may change.
- Every lesson has at least one goal of a type defined in [ADR-0007](../adr/0007-challenge-goal-types.md): **state** (assertions on simulated resources), **answer** (learner answers a question; the expected answer is computed server-side from session state or fixed in the hidden spec), or **observation** (a successful command of a given *parsed* shape ran in this session).
- Every command and behavior listed is in [simulator-compatibility.md](../architecture/simulator-compatibility.md). The "Simulator behaviors" column names the behaviors from that document the lesson depends on.
- Every lesson ships a hidden reference solution that CI replays in the simulator to prove the lesson is solvable (Part 17).

Legend for goal types: **S** = state, **A** = answer, **O** = observation.

## 1. Kubernetes fundamentals — `fundamentals` (beginner)

| # | Lesson ID | Title | Key commands | Goals | Simulator behaviors |
| --- | --- | --- | --- | --- | --- |
| 1 | `fundamentals/cluster-architecture` | How a cluster is put together | `get nodes`, `get pods -n kube-system`, `cluster-info` | A: number of nodes (count); A: which component stores cluster state (choice) | Seeded nodes; seeded read-only control-plane Pods in `kube-system` |
| 2 | `fundamentals/namespace-navigation` | Finding your way with namespaces | `get ns`, `get pods -n`, `-A`, `config set-context --current --namespace` | O: `config set-context` set namespace `team-a`; A: namespace containing Pod `billing-api` (path) | Simulated kubeconfig context |
| 3 | `fundamentals/listing-resources` | Listing and reading resources | `get`, `-o wide`, `-o yaml`, `get all` | A: node that Pod `web-0` runs on (path); A: number of Pods in `shop` (count) | Scheduler placement; output formats |
| 4 | `fundamentals/labels-and-selectors` | Labels and selectors | `get -l`, `get --show-labels`, `label` | S: Pod `cart` has `tier=backend`; A: Pods matching `app=shop,tier=frontend` (count) | Label selectors (equality and set-based) |
| 5 | `fundamentals/declarative-vs-imperative` | Imperative commands vs. declarative manifests | `create deployment`, YAML editor, `apply -f` | S: Deployment `catalog` exists with 2 replicas **and** carries the `kubectl.kubernetes.io/last-applied-configuration` annotation (proves `apply`) | Apply annotation; Deployment controller |

## 2. Pods and workloads — `workloads` (beginner)

| # | Lesson ID | Title | Key commands | Goals | Simulator behaviors |
| --- | --- | --- | --- | --- | --- |
| 6 | `workloads/launch-a-pod` | Launch your first Pod | `run`, `get pods` | S: Pod `web` exists, image `nginx:1.27`, phase `Running` | Image catalog; kubelet simulation |
| 7 | `workloads/inspect-a-pod` | Inspect a running Pod | `describe pod`, `logs`, `get pod -o yaml` | O: `describe` on Pod `api`; A: Pod IP of `api` (path); A: first log line keyword (static) | Synthetic logs; IP allocation |
| 8 | `workloads/troubleshoot-pending-pod` | Why is my Pod Pending? | `describe`, `get events`, `label node` or edit manifest | S: Deployment `reports` `status.readyReplicas` = 2 | Scheduler `nodeSelector` filtering; `FailedScheduling` events |
| 9 | `workloads/create-a-deployment` | Create a Deployment | `create deployment`, `get deploy,rs,pods` | S: Deployment `shop` image `nginx:1.27`, `spec.replicas` = 3, `status.readyReplicas` = 3 | Deployment → ReplicaSet → Pod controllers |
| 10 | `workloads/scaling-deployments` | Scale a Deployment | `scale`, or edit + `apply` | S: Deployment `nginx` `spec.replicas` = 3 and `status.readyReplicas` = 3 | ReplicaSet scale up/down |
| 11 | `workloads/rollout-and-rollback` | Roll out and roll back | `set image`, `rollout status`, `rollout history`, `rollout undo` | S: Deployment `web` image back to `nginx:1.26` **and** revision annotation ≥ 3 (proves a rollout then an undo) | Rolling update; revision history; `rollout undo` |

## 3. Networking — `networking` (beginner → intermediate)

| # | Lesson ID | Title | Key commands | Goals | Simulator behaviors |
| --- | --- | --- | --- | --- | --- |
| 12 | `networking/service-basics` | What a Service is | `get svc`, `describe svc`, `get endpointslices` | A: ClusterIP of Service `web` (path); A: number of endpoints behind `web` (count) | ClusterIP allocation; EndpointSlice controller |
| 13 | `networking/expose-a-deployment` | Expose a Deployment | `expose`, `create service` | S: Service `shop` selects `app=shop`, port 80 → targetPort 8080; S: its EndpointSlice has 3 ready endpoints | Service + EndpointSlice controllers |
| 14 | `networking/selectors-and-endpoints` | How Services find Pods | `get pods --show-labels`, `label`, `get endpointslices` | S: EndpointSlice for `api` has 3 endpoints (learner labels the missing Pod into the selector) | Label selectors; EndpointSlice updates |
| 15 | `networking/clusterip-vs-nodeport` | ClusterIP vs. NodePort | `edit` via YAML editor + `apply`, `get svc` | S: Service `web` type `NodePort` with a `nodePort` assigned; A: the allocated nodePort (path) | NodePort allocation (concept: no real traffic) |
| 16 | `networking/debug-missing-endpoints` | Debug a Service with no endpoints | `describe svc`, `get endpointslices`, `get pods --show-labels` | S: EndpointSlice for `checkout` has 2 ready endpoints | Selector mismatch → empty EndpointSlice |

## 4. Configuration — `configuration` (intermediate)

| # | Lesson ID | Title | Key commands | Goals | Simulator behaviors |
| --- | --- | --- | --- | --- | --- |
| 17 | `configuration/configmaps` | Store configuration in a ConfigMap | `create configmap --from-literal`, `get cm -o yaml` | S: ConfigMap `app-config` has `data.LOG_LEVEL` = `debug` and `data.FEATURE_X` = `on` | ConfigMap validation |
| 18 | `configuration/editing-manifests` | Edit and re-apply a manifest | YAML editor, `apply -f`, `diff` is **not** supported (explained) | S: Deployment `api` has label `team=payments` on the template and `containerPort` 8080 | Apply merge semantics (documented subset) |
| 19 | `configuration/environment-from-configmaps` | Feed environment variables from a ConfigMap | `describe pod`, `create configmap` | S: Deployment `worker` `status.readyReplicas` = 2 (Pods were stuck in `CreateContainerConfigError` until the ConfigMap existed) | Env reference resolution; `CreateContainerConfigError` |
| 20 | `configuration/secrets-awareness` | Secrets are not encryption | `create secret generic --from-literal`, `get secret -o yaml` | S: Secret `db-creds` type `Opaque` has key `username`; A: decoded value of the training Secret `demo-token` (static) | Opaque Secrets with dummy values; base64 encoding |

## 5. Cluster operations — `cluster-operations` (intermediate)

| # | Lesson ID | Title | Key commands | Goals | Simulator behaviors |
| --- | --- | --- | --- | --- | --- |
| 21 | `cluster-operations/inspect-nodes` | Inspect Nodes | `get nodes -o wide`, `describe node` | A: allocatable CPU of `node-2` (path); A: number of nodes labelled `disk=ssd` (count) | Seeded node capacity, labels, conditions |
| 22 | `cluster-operations/namespaces-and-scope` | Namespaced vs. cluster-scoped | `api-resources --namespaced`, `create namespace` | S: Namespace `staging` exists; A: "Is a Node namespaced?" (choice) | `api-resources` table; Namespace lifecycle |
| 23 | `cluster-operations/labels-and-annotations` | Labels vs. annotations | `label`, `annotate`, `--overwrite` | S: Deployment `search` has label `env=prod` and annotation `owner=platform-team` | Label/annotation validation and `--overwrite` |
| 24 | `cluster-operations/requests-limits-scheduling` | Requests, limits and scheduling | `describe node`, YAML editor + `apply` | S: Deployment `batch` `status.readyReplicas` = 3 and container CPU request ≤ `500m` | Resource-fit scheduling against allocatable |
| 25 | `cluster-operations/events-and-diagnostics` | Read events like a detective | `get events`, `describe` | A: reason of the Warning event for Pod `etl` (choice); O: `get events` succeeded | Synthetic events with reasons |

## 6. Troubleshooting — `troubleshooting` (intermediate)

| # | Lesson ID | Title | Key commands | Goals | Simulator behaviors |
| --- | --- | --- | --- | --- | --- |
| 26 | `troubleshooting/image-pull-failure` | Fix an image pull failure | `describe pod`, `get events`, `set image` or `apply` | S: Deployment `frontend` image `nginx:1.27`, `status.readyReplicas` = 2 | Unknown image → `ErrImagePull`/`ImagePullBackOff` |
| 27 | `troubleshooting/crashloopbackoff` | Fix a CrashLoopBackOff | `logs`, `logs --previous`, `describe`, YAML editor | S: Deployment `orders` `status.readyReplicas` = 1 (crash profile requires env `APP_MODE`) | Crash profiles; restart backoff on simulated time |
| 28 | `troubleshooting/wrong-labels` | A Deployment whose Pods nobody can reach | `get pods --show-labels`, `describe svc`, `apply` | S: EndpointSlice for `profile` has 2 ready endpoints **and** Service selector unchanged (fix must be on the Deployment template) | Selector/label mismatch; template change triggers rollout |
| 29 | `troubleshooting/replica-mismatch` | Desired 4, ready 1 | `get deploy`, `describe rs`, `describe node`, `apply` | S: Deployment `render` `status.readyReplicas` = 4 | Insufficient-CPU scheduling; ReplicaSet status |
| 30 | `troubleshooting/capstone-recovery` | Capstone: bring the shop back online | Everything above | S: Deployments `shop-web`, `shop-api`, `shop-worker` fully ready; S: EndpointSlices for `shop-web`, `shop-api` non-empty; S: ConfigMap `shop-config` has `API_URL` | Combines image, selector and config faults |

## Coverage check

- Goal type use: state only 19; answer/observation only 7 (lessons 1, 2, 3, 7, 12, 21, 25); state plus answer 4 (lessons 4, 15, 20, 22). All 30 have at least one gradable goal, and no lesson can be passed without doing anything: every answer goal's expected value comes from the session state or hidden spec, and every lesson with only read-type goals has an answer the learner must find in the cluster.
- Kinds touched: Namespace, Node, Pod, Deployment, ReplicaSet, Service, EndpointSlice, ConfigMap, Secret, Event. No lesson needs a kind marked *not supported*.
- Behaviors needed only by one lesson (prioritize review of these): kubeconfig context (2), `last-applied-configuration` (5), revision history (11), NodePort allocation (15), env reference errors (19), crash profiles (27).
