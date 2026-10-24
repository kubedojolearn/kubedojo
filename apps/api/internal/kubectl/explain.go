package kubectl

import (
	"fmt"
	"strings"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// field is one documented field of a supported kind (the simulator's subset of the OpenAPI schema).
type field struct {
	name     string
	typ      string
	desc     string
	children []field
}

func f(name, typ, desc string, children ...field) field {
	return field{name: name, typ: typ, desc: desc, children: children}
}

var (
	apiVersionField = f("apiVersion", "string", "APIVersion defines the versioned schema of this representation of an object.")
	kindField       = f("kind", "string", "Kind is a string value representing the REST resource this object represents.")
	metadataField   = f("metadata", "ObjectMeta", "Standard object's metadata.",
		f("name", "string", "Name must be unique within a namespace."),
		f("namespace", "string", "Namespace defines the space within which each name must be unique."),
		f("labels", "map[string]string", "Map of string keys and values that can be used to organize and categorize objects. Selectors match on labels."),
		f("annotations", "map[string]string", "Unstructured key value map for arbitrary metadata."),
		f("uid", "string", "UID is the unique in time and space value for this object. Set by the system."),
		f("resourceVersion", "string", "An opaque value that represents the internal version of this object. Set by the system."),
		f("generation", "integer", "A sequence number representing a specific generation of the desired state. Set by the system."),
		f("creationTimestamp", "string", "Time when this object was created. Set by the system."),
		f("ownerReferences", "[]OwnerReference", "Objects this object depends on. Dependents are garbage collected when their controlling owner is deleted."),
	)
	containerField = f("containers", "[]Container", "List of containers belonging to the pod. There must be at least one.",
		f("name", "string", "Name of the container, unique within the pod. Required."),
		f("image", "string", "Container image name. In the simulator, images resolve against a built-in catalog."),
		f("command", "[]string", "Entrypoint array. Not executed within a shell."),
		f("args", "[]string", "Arguments to the entrypoint."),
		f("env", "[]EnvVar", "List of environment variables to set in the container.",
			f("name", "string", "Name of the environment variable. Required."),
			f("value", "string", "The variable's value."),
			f("valueFrom", "EnvVarSource", "Source for the variable's value: configMapKeyRef or secretKeyRef."),
		),
		f("envFrom", "[]EnvFromSource", "Sources to populate environment variables in the container: configMapRef or secretRef."),
		f("ports", "[]ContainerPort", "List of ports to expose from the container.",
			f("containerPort", "integer", "Number of port to expose on the pod's IP address. Required."),
			f("name", "string", "A name for the port that Services can refer to."),
			f("protocol", "string", "Protocol for port. Defaults to TCP."),
		),
		f("resources", "ResourceRequirements", "Compute resources required by this container.",
			f("requests", "map[string]Quantity", "Minimum resources required (cpu, memory). The scheduler uses requests."),
			f("limits", "map[string]Quantity", "Maximum resources allowed (cpu, memory)."),
		),
	)
	podSpecField = f("spec", "PodSpec", "Specification of the desired behavior of the pod.",
		containerField,
		f("nodeSelector", "map[string]string", "The pod only fits on nodes whose labels match all of these."),
		f("tolerations", "[]Toleration", "The pod's tolerations of node taints."),
		f("restartPolicy", "string", "Restart policy for all containers. The simulator supports Always."),
		f("nodeName", "string", "The node the pod is bound to. Set by the scheduler."),
	)
	templateField = f("template", "PodTemplateSpec", "Template describes the pods that will be created.",
		f("metadata", "ObjectMeta", "Labels and annotations of created pods. Labels must match the selector."),
		podSpecField,
	)
	selectorField = f("selector", "LabelSelector", "Label selector for pods. Must match the template's labels. Immutable for deployments.",
		f("matchLabels", "map[string]string", "Labels that must all match."),
		f("matchExpressions", "[]LabelSelectorRequirement", "Set-based requirements: In, NotIn, Exists, DoesNotExist."),
	)
	replicasField = f("replicas", "integer", "Number of desired pods. Defaults to 1.")
)

var explainSchema = map[string]field{
	simulator.KindPod: f("Pod", "", "Pod is a collection of containers that can run on a host. This resource is created by clients and scheduled onto hosts.",
		apiVersionField, kindField, metadataField, podSpecField,
		f("status", "PodStatus", "Most recently observed status of the pod. Read-only.",
			f("phase", "string", "Pending, Running, Succeeded, Failed or Unknown."),
			f("conditions", "[]PodCondition", "Current service state of the pod."),
			f("podIP", "string", "IP address allocated to the pod."),
			f("hostIP", "string", "IP address of the node the pod is running on."),
			f("startTime", "string", "When the pod was acknowledged by the kubelet."),
			f("containerStatuses", "[]ContainerStatus", "Status of each container: state, lastState, ready, restartCount."),
		)),
	simulator.KindDeployment: f("Deployment", "", "Deployment enables declarative updates for Pods and ReplicaSets.",
		apiVersionField, kindField, metadataField,
		f("spec", "DeploymentSpec", "Specification of the desired behavior of the Deployment.",
			replicasField, selectorField, templateField,
			f("strategy", "DeploymentStrategy", "The deployment strategy to use to replace existing pods with new ones.",
				f("type", "string", "RollingUpdate (default) or Recreate."),
				f("rollingUpdate", "RollingUpdateDeployment", "Rolling update config params.",
					f("maxSurge", "IntOrString", "Pods that can be scheduled above the desired number. Defaults to 25%."),
					f("maxUnavailable", "IntOrString", "Pods that can be unavailable during the update. Defaults to 25%."),
				),
			),
			f("revisionHistoryLimit", "integer", "Number of old ReplicaSets to retain for rollback. Defaults to 10."),
		),
		f("status", "DeploymentStatus", "Most recently observed status of the Deployment. Read-only.",
			f("replicas", "integer", "Total number of pods targeted by this deployment."),
			f("updatedReplicas", "integer", "Pods that have the desired template."),
			f("readyReplicas", "integer", "Pods that are ready."),
			f("availableReplicas", "integer", "Pods that are available."),
			f("conditions", "[]DeploymentCondition", "Available and Progressing conditions."),
		)),
	simulator.KindReplicaSet: f("ReplicaSet", "", "ReplicaSet ensures that a specified number of pod replicas are running at any given time.",
		apiVersionField, kindField, metadataField,
		f("spec", "ReplicaSetSpec", "Spec defines the desired behavior of the ReplicaSet.", replicasField, selectorField, templateField),
		f("status", "ReplicaSetStatus", "Most recently observed status of the ReplicaSet. Read-only.",
			f("replicas", "integer", "Most recently observed number of replicas."),
			f("readyReplicas", "integer", "Number of ready replicas."),
		)),
	simulator.KindService: f("Service", "", "Service is a named abstraction of software service consisting of a port that the proxy listens on, and the selector that determines which pods will answer requests sent through the proxy.",
		apiVersionField, kindField, metadataField,
		f("spec", "ServiceSpec", "Spec defines the behavior of a service.",
			f("type", "string", "ClusterIP (default), NodePort or LoadBalancer. In the simulator, LoadBalancer stays pending."),
			f("selector", "map[string]string", "Route service traffic to pods with label keys and values matching this selector."),
			f("ports", "[]ServicePort", "The list of ports exposed by this service.",
				f("port", "integer", "The port that will be exposed by this service. Required."),
				f("targetPort", "IntOrString", "Number or name of the port to access on the pods. Defaults to port."),
				f("nodePort", "integer", "The port on each node for NodePort services (30000-32767). Allocated if unset."),
				f("name", "string", "The name of this port within the service."),
				f("protocol", "string", "TCP (default)."),
			),
			f("clusterIP", "string", "The IP address of the service. Assigned by the cluster."),
		)),
	simulator.KindConfigMap: f("ConfigMap", "", "ConfigMap holds configuration data for pods to consume.",
		apiVersionField, kindField, metadataField,
		f("data", "map[string]string", "Data contains the configuration data.")),
	simulator.KindSecret: f("Secret", "", "Secret holds secret data of a certain type. In the simulator, use training values only.",
		apiVersionField, kindField, metadataField,
		f("type", "string", "Used to facilitate programmatic handling of secret data. The simulator supports Opaque."),
		f("data", "map[string]string", "Secret data, base64 encoded."),
		f("stringData", "map[string]string", "Write-only convenience field: plain values merged into data.")),
	simulator.KindNamespace: f("Namespace", "", "Namespace provides a scope for Names.",
		apiVersionField, kindField, metadataField,
		f("status", "NamespaceStatus", "Status describes the current status of a Namespace.",
			f("phase", "string", "Phase is the current lifecycle phase of the namespace."))),
	simulator.KindNode: f("Node", "", "Node is a worker node in Kubernetes. In the simulator, nodes come from the lab's fixture.",
		apiVersionField, kindField, metadataField,
		f("spec", "NodeSpec", "Spec defines the behavior of a node.",
			f("taints", "[]Taint", "If specified, the node's taints."),
			f("unschedulable", "boolean", "Controls node schedulability of new pods.")),
		f("status", "NodeStatus", "Most recently observed status of the node.",
			f("capacity", "map[string]Quantity", "Total resources of the node."),
			f("allocatable", "map[string]Quantity", "Resources available for scheduling."),
			f("conditions", "[]NodeCondition", "Conditions is an array of current observed node conditions."),
			f("addresses", "[]NodeAddress", "List of addresses reachable to the node."))),
	simulator.KindEndpointSlice: f("EndpointSlice", "", "EndpointSlice represents a subset of the endpoints that implement a service.",
		apiVersionField, kindField, metadataField,
		f("addressType", "string", "The type of address carried by this EndpointSlice."),
		f("endpoints", "[]Endpoint", "A list of unique endpoints in this slice."),
		f("ports", "[]EndpointPort", "The network ports exposed by each endpoint.")),
}

// unsupportedFields are real Kubernetes fields the simulator doesn't model.
var unsupportedFields = map[string]bool{
	"volumes": true, "affinity": true, "initContainers": true, "securityContext": true, "livenessProbe": true,
	"readinessProbe": true, "startupProbe": true, "volumeMounts": true, "serviceAccountName": true,
	"hostNetwork": true, "priorityClassName": true, "topologySpreadConstraints": true, "paused": true,
	"progressDeadlineSeconds": true, "minReadySeconds": true, "externalName": true, "sessionAffinity": true,
	"binaryData": true, "immutable": true, "finalizers": true,
}

func runExplain(args []string, _ Env, p *Parsed) (Result, *Failure) {
	pos, fl, fail := parseFlags(args, "explain", []flagDef{{Long: "recursive"}, {Long: "api-version", Value: true}})
	if fail != nil {
		return Result{}, fail
	}
	if r, ok := helpRequested(fl, "explain"); ok {
		return r, nil
	}
	recordFlags(p, fl)
	if len(pos) != 1 {
		return Result{}, reject(CodeValidation, "You must specify the type of resource to explain. Use \"kubectl api-resources\" for a complete list of supported resources.\n\nerror: Required resource not specified.")
	}
	parts := strings.Split(pos[0], ".")
	// "deployments.apps.spec" style group qualifiers are skipped.
	t, fail := lookupType(parts[0])
	if fail != nil {
		return Result{}, fail
	}
	rest := parts[1:]
	if t.Group != "" && len(rest) > 0 && strings.HasPrefix(t.Group, rest[0]) {
		rest = rest[len(strings.Split(t.Group, ".")):]
	}
	root, ok := explainSchema[t.Kind]
	if !ok {
		return Result{}, reject(CodeUnsupported, "error: the simulator has no field documentation for %s", t.display())
	}
	p.Resources = []string{t.Kind}
	cur := root
	for _, name := range rest {
		var next *field
		for i := range cur.children {
			if cur.children[i].name == name {
				next = &cur.children[i]
			}
		}
		if next == nil {
			if unsupportedFields[name] {
				return Result{}, reject(CodeUnsupported, "error: field %q exists in Kubernetes but isn't supported by the simulator. See 'kubectl explain %s' for the supported fields.", name, strings.ToLower(t.Kind))
			}
			return Result{}, reject(CodeNotFound, "error: field %q does not exist", name)
		}
		cur = *next
	}

	var b strings.Builder
	fmt.Fprintf(&b, "KIND:       %s\n", t.Kind)
	if t.Group != "" {
		fmt.Fprintf(&b, "GROUP:      %s\n", t.Group)
	}
	fmt.Fprintf(&b, "VERSION:    %s\n\n", t.Version)
	if len(rest) > 0 {
		fmt.Fprintf(&b, "FIELD: %s <%s>\n\n", cur.name, cur.typ)
	}
	fmt.Fprintf(&b, "DESCRIPTION:\n%s\n", indentText(cur.desc, "    "))
	if len(cur.children) > 0 {
		b.WriteString("    \nFIELDS:\n")
		if fl.boolean("recursive") {
			writeRecursive(&b, cur.children, 1)
		} else {
			for _, c := range cur.children {
				fmt.Fprintf(&b, "  %s\t<%s>\n%s\n\n", c.name, c.typ, indentText(c.desc, "    "))
			}
		}
	}
	return Result{Stdout: strings.TrimRight(b.String(), "\n") + "\n"}, nil
}

func writeRecursive(b *strings.Builder, fields []field, level int) {
	for _, c := range fields {
		fmt.Fprintf(b, "%s%s\t<%s>\n", strings.Repeat("  ", level), c.name, c.typ)
		writeRecursive(b, c.children, level+1)
	}
}

// indentText wraps text at 76 columns with the given indent, like kubectl explain.
func indentText(s, indent string) string {
	var lines []string
	line := ""
	for _, w := range strings.Fields(s) {
		if line != "" && len(indent)+len(line)+1+len(w) > 80 {
			lines = append(lines, indent+line)
			line = w
			continue
		}
		if line == "" {
			line = w
		} else {
			line += " " + w
		}
	}
	if line != "" {
		lines = append(lines, indent+line)
	}
	return strings.Join(lines, "\n")
}
