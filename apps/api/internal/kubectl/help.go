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

Common flags:
  -n, --namespace <ns>   Work in another namespace
  -A, --all-namespaces   List across all namespaces
  -l, --selector <sel>   Filter by labels, e.g. -l app=web or -l 'tier in (web,api)'
  -o, --output <fmt>     wide, yaml, json or name
  -h, --help             Help for a command: kubectl get --help

Changing the cluster (create, apply, scale, delete, ...) arrives in the next update.
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
	"rollout": `Inspect Deployment rollouts.

Examples:
  kubectl rollout status deployment/nginx
  kubectl rollout history deployment/nginx
  kubectl rollout history deployment/nginx --revision=2

(rollout undo and restart arrive with commands that change the cluster.)
`,
	"config": `Show the simulated kubeconfig.

Examples:
  kubectl config view
  kubectl config get-contexts
  kubectl config current-context
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
