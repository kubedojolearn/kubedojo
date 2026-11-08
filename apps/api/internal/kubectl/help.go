package kubectl

const helpText = `kubectl runs against your simulated KubeDojo cluster. Nothing runs on a real cluster.

Inspect the cluster:
  get            List resources: kubectl get pods, kubectl get deploy,svc -o wide
  describe       Show details and events: kubectl describe pod <name>
  logs           Print a container's (simulated) logs: kubectl logs <pod>
  events         List events: kubectl events --types=Warning
  explain        Document a resource's fields: kubectl explain deployment.spec
  rollout        Check a rollout: kubectl rollout status deployment/<name>
  api-resources  List the resource types the simulator has
  cluster-info   Show the (simulated) control plane address
  version        Show the client and server versions
  config         Show the kubeconfig: kubectl config view

Change the cluster:
  create         Create a deployment, namespace, configmap, secret or service
  run            Start a Pod: kubectl run web --image=nginx:1.27
  expose         Create a Service for a deployment or Pod: kubectl expose deploy web --port=80
  scale          Change replicas: kubectl scale deployment web --replicas=3
  set            Change an image or env: kubectl set image deployment/web nginx=nginx:1.27
  rollout        Undo or restart a rollout: kubectl rollout undo deployment/web
  label          Add or remove labels: kubectl label pod web tier=frontend
  annotate       Add or remove annotations
  delete         Delete resources: kubectl delete pod web
  config set-context --current --namespace=<ns>   Change your default namespace

Common flags:
  -n, --namespace <ns>   Work in another namespace
  -A, --all-namespaces   List across all namespaces
  -l, --selector <sel>   Filter by labels, e.g. -l app=web or -l 'tier in (web,api)'
  -o, --output <fmt>     wide, yaml, json or name
  -h, --help             Help for a command: kubectl get --help
      --dry-run=client   Show what create/run/expose would make, without making it

Every change moves the simulated clock forward 10 seconds, and the controllers then bring
the cluster to its new desired state before the command returns.
Shell features (pipes, ;, &&, redirection, $variables) aren't available: there is no shell.
Type 'clear' to clear the terminal.
`

var commandHelp = map[string]string{
	"get": `Display one or many resources.

Examples:
  kubectl get pods
  kubectl get pods -o wide
  kubectl get deployment nginx -o yaml
  kubectl get pods,services -n kube-system
  kubectl get all -A
  kubectl get pods -l app=nginx --show-labels

Options:
  -A, --all-namespaces   List across all namespaces
  -l, --selector         Label selector (=, ==, !=, in, notin, exists, !key)
  -o, --output           wide | yaml | json | name
      --show-labels      Add a LABELS column
      --no-headers       Omit the header row
      --sort-by          metadata.name or metadata.creationTimestamp

Resource types: pods (po), deployments (deploy), replicasets (rs), services (svc),
namespaces (ns), nodes (no), configmaps (cm), secrets, endpointslices, endpoints (ep),
events (ev), all
`,
	"describe": `Show details of a resource, including recent events.

Examples:
  kubectl describe pod nginx-7c5ddbdf54-abcde
  kubectl describe deployment nginx
  kubectl describe pods -l app=nginx
  kubectl describe nodes
`,
	"logs": `Print the logs of a container in a Pod. Logs are simulated from the image's profile.

Examples:
  kubectl logs nginx-7c5ddbdf54-abcde
  kubectl logs nginx-7c5ddbdf54-abcde -c nginx
  kubectl logs nginx-7c5ddbdf54-abcde --previous
  kubectl logs deployment/nginx --tail=5

Options:
  -c, --container   Container name (required when the Pod has several)
  -p, --previous    Logs of the previous, crashed container
      --tail        Number of recent lines to show
`,
	"events": `List events, oldest first.

Examples:
  kubectl events
  kubectl events -A
  kubectl events --for pod/nginx-7c5ddbdf54-abcde
  kubectl events --types=Warning

Options:
      --for     Only events about this object (kind/name)
      --types   Comma-separated event types: Normal, Warning
`,
	"explain": `Describe the fields of a supported resource.

Examples:
  kubectl explain pods
  kubectl explain deployment.spec.replicas
  kubectl explain service.spec --recursive
`,
	"rollout": `Manage Deployment rollouts.

Examples:
  kubectl rollout status deployment/nginx
  kubectl rollout history deployment/nginx
  kubectl rollout history deployment/nginx --revision=2
  kubectl rollout undo deployment/nginx
  kubectl rollout undo deployment/nginx --to-revision=1
  kubectl rollout restart deployment/nginx
`,
	"config": `Show the simulated kubeconfig, or change its default namespace.

Examples:
  kubectl config view
  kubectl config get-contexts
  kubectl config current-context
  kubectl config set-context --current --namespace=shop
`,
	"create": `Create a resource.

Examples:
  kubectl create deployment web --image=nginx:1.27 --replicas=3 --port=80
  kubectl create namespace shop
  kubectl create configmap settings --from-literal=mode=prod
  kubectl create secret generic db --from-literal=password=practice-only
  kubectl create service nodeport web --tcp=80:80
  kubectl create deployment web --image=nginx:1.27 --dry-run=client -o yaml

Options:
      --dry-run=client   Print the object instead of creating it
  -o, --output           yaml | json | name
`,
	"run": `Create a Pod running one container.

Examples:
  kubectl run web --image=nginx:1.27
  kubectl run web --image=nginx:1.27 --port=80 --labels=app=web,tier=frontend
  kubectl run web --image=nginx:1.27 --env=MODE=prod --dry-run=client -o yaml

The simulator doesn't attach to containers, so -it, --rm and commands after -- aren't available.
`,
	"expose": `Create a Service that selects the Pods of a deployment, replicaset, Pod or Service.

Examples:
  kubectl expose deployment web --port=80
  kubectl expose deployment web --port=80 --target-port=8080 --type=NodePort
  kubectl expose pod web --port=80 --name=web-svc

Options:
      --port          Service port (defaults to the containers' ports)
      --target-port   Container port to send traffic to (defaults to --port)
      --type          ClusterIP | NodePort | LoadBalancer
      --name          Service name (defaults to the resource's name)
      --selector      Pods to select (defaults to the resource's selector or labels)
`,
	"scale": `Set a new size for a deployment or replicaset.

Examples:
  kubectl scale deployment web --replicas=3
  kubectl scale deployment/web --replicas=1 --current-replicas=3
  kubectl scale deployments -l tier=web --replicas=2
`,
	"set": `Change a workload's Pod template, which starts a rollout.

Examples:
  kubectl set image deployment/web nginx=nginx:1.27
  kubectl set image deployment/web '*=nginx:1.27'
  kubectl set env deployment/web MODE=prod LEVEL-
  kubectl set env deployment/web --from=configmap/settings
`,
	"label": `Add, change (--overwrite) or remove (KEY-) labels.

Examples:
  kubectl label pod web tier=frontend
  kubectl label pod web tier=backend --overwrite
  kubectl label pod web tier-
  kubectl label pods -l app=web env=lab
  kubectl label node node-2 disktype=ssd
`,
	"annotate": `Add, change (--overwrite) or remove (KEY-) annotations.

Examples:
  kubectl annotate deployment web owner=platform
  kubectl annotate deployment web owner-
`,
	"delete": `Delete resources by name, label selector or --all.

Examples:
  kubectl delete pod web
  kubectl delete deployment web
  kubectl delete pods -l app=web
  kubectl delete configmaps --all
  kubectl delete deployment web --cascade=orphan

Deletion is immediate in the simulator. Deleting an owner deletes what it owns unless you
pass --cascade=orphan.
`,
	"api-resources": `List the resource types the simulated cluster serves.

Options:
      --namespaced   true or false
  -o, --output       wide or name
`,
	"cluster-info": "Show the address of the simulated control plane.\n",
	"version":      "Print the client and server versions.\n\nOptions:\n  -o, --output   yaml or json\n",
	"api-versions": "Print the API versions the simulated cluster serves.\n",
	"help":         helpText,
}
