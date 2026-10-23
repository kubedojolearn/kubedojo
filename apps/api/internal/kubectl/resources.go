package kubectl

import (
	"strings"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// Pseudo-kinds served by kubectl but not stored as simulator objects.
const (
	kindEvent     = "Event"
	kindEndpoints = "Endpoints"
)

// resourceType is one entry of `kubectl api-resources`, limited to what the simulator serves.
type resourceType struct {
	Kind       string
	Plural     string
	Singular   string
	Short      []string
	Group      string // "" for the core group
	Version    string
	Namespaced bool
}

// qualified is the name kubectl prints in `-o name` and multi-type listings: pod, deployment.apps.
func (t resourceType) qualified() string {
	if t.Group == "" {
		return t.Singular
	}
	return t.Singular + "." + t.Group
}

// display is the plural form used in error messages: pods, deployments.apps.
func (t resourceType) display() string {
	if t.Group == "" {
		return t.Plural
	}
	return t.Plural + "." + t.Group
}

func (t resourceType) apiVersion() string {
	if t.Group == "" {
		return t.Version
	}
	return t.Group + "/" + t.Version
}

// resourceTypes in `kubectl api-resources` order (core group first, then by group).
var resourceTypes = []resourceType{
	{Kind: simulator.KindConfigMap, Plural: "configmaps", Singular: "configmap", Short: []string{"cm"}, Version: "v1", Namespaced: true},
	{Kind: kindEndpoints, Plural: "endpoints", Singular: "endpoints", Short: []string{"ep"}, Version: "v1", Namespaced: true},
	{Kind: kindEvent, Plural: "events", Singular: "event", Short: []string{"ev"}, Version: "v1", Namespaced: true},
	{Kind: simulator.KindNamespace, Plural: "namespaces", Singular: "namespace", Short: []string{"ns"}, Version: "v1"},
	{Kind: simulator.KindNode, Plural: "nodes", Singular: "node", Short: []string{"no"}, Version: "v1"},
	{Kind: simulator.KindPod, Plural: "pods", Singular: "pod", Short: []string{"po"}, Version: "v1", Namespaced: true},
	{Kind: simulator.KindSecret, Plural: "secrets", Singular: "secret", Version: "v1", Namespaced: true},
	{Kind: simulator.KindService, Plural: "services", Singular: "service", Short: []string{"svc"}, Version: "v1", Namespaced: true},
	{Kind: simulator.KindDeployment, Plural: "deployments", Singular: "deployment", Short: []string{"deploy"}, Group: "apps", Version: "v1", Namespaced: true},
	{Kind: simulator.KindReplicaSet, Plural: "replicasets", Singular: "replicaset", Short: []string{"rs"}, Group: "apps", Version: "v1", Namespaced: true},
	{Kind: simulator.KindEndpointSlice, Plural: "endpointslices", Singular: "endpointslice", Group: "discovery.k8s.io", Version: "v1", Namespaced: true},
}

// unsupportedTypes are real Kubernetes resources the simulator doesn't have, so the learner gets
// "not simulated" rather than "doesn't exist".
var unsupportedTypes = map[string]string{}

func init() {
	for _, names := range [][]string{
		{"statefulsets", "statefulset", "sts"}, {"daemonsets", "daemonset", "ds"}, {"jobs", "job"},
		{"cronjobs", "cronjob", "cj"}, {"ingresses", "ingress", "ing"}, {"persistentvolumes", "persistentvolume", "pv"},
		{"persistentvolumeclaims", "persistentvolumeclaim", "pvc"}, {"storageclasses", "storageclass", "sc"},
		{"serviceaccounts", "serviceaccount", "sa"}, {"roles", "role"}, {"rolebindings", "rolebinding"},
		{"clusterroles", "clusterrole"}, {"clusterrolebindings", "clusterrolebinding"},
		{"networkpolicies", "networkpolicy", "netpol"}, {"horizontalpodautoscalers", "horizontalpodautoscaler", "hpa"},
		{"resourcequotas", "resourcequota", "quota"}, {"limitranges", "limitrange", "limits"},
		{"poddisruptionbudgets", "poddisruptionbudget", "pdb"}, {"customresourcedefinitions", "customresourcedefinition", "crd", "crds"},
		{"gateways", "gateway"}, {"httproutes", "httproute"}, {"leases", "lease"}, {"priorityclasses", "priorityclass", "pc"},
		{"replicationcontrollers", "replicationcontroller", "rc"}, {"podtemplates", "podtemplate"},
		{"componentstatuses", "componentstatus", "cs"}, {"csinodes", "csinode"}, {"volumeattachments", "volumeattachment"},
	} {
		for _, n := range names {
			unsupportedTypes[n] = names[0]
		}
	}
}

// lookupType resolves a resource argument (plural, singular, short name, Kind, optionally with
// its group: deploy, deployments.apps, Deployment).
func lookupType(arg string) (resourceType, *Failure) {
	name := strings.ToLower(arg)
	base, group, hasGroup := strings.Cut(name, ".")
	for _, t := range resourceTypes {
		if hasGroup && group != t.Group && group != t.Group+"/v1" {
			continue
		}
		if base == t.Plural || base == t.Singular || base == strings.ToLower(t.Kind) {
			return t, nil
		}
		for _, s := range t.Short {
			if base == s && !hasGroup {
				return t, nil
			}
		}
	}
	if plural, ok := unsupportedTypes[base]; ok {
		return resourceType{}, reject(CodeUnsupported, "error: %s are not part of the KubeDojo simulator yet. Run 'kubectl api-resources' to see what is.", plural)
	}
	return resourceType{}, reject(CodeNotFound, "error: the server doesn't have a resource type %q", arg)
}

func typeOfKind(kind string) resourceType {
	for _, t := range resourceTypes {
		if t.Kind == kind {
			return t
		}
	}
	return resourceType{Kind: kind, Plural: strings.ToLower(kind) + "s", Singular: strings.ToLower(kind), Version: "v1"}
}
