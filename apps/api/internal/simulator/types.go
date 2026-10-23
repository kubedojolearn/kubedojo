// Package simulator is KubeDojo's deterministic, pure model of a Kubernetes v1.37 subset
// (docs/architecture/simulator-compatibility.md). It performs no I/O, reads no wall clock and
// uses no global randomness: every output is a function of the input state, the session seed
// and the logical clock (ADR-0006). The linter enforces the import rules.
package simulator

import (
	"encoding/json"
	"time"
)

// Version identifies the behavior implemented here. Sessions created by another version are
// not reinterpreted (see labs).
const Version = "kubernetes-1.37-subset-1"

// StateSchemaVersion is the serialization format of stored objects.
const StateSchemaVersion = 1

// Kinds supported by the simulator.
const (
	KindNamespace     = "Namespace"
	KindNode          = "Node"
	KindPod           = "Pod"
	KindDeployment    = "Deployment"
	KindReplicaSet    = "ReplicaSet"
	KindService       = "Service"
	KindEndpointSlice = "EndpointSlice"
	KindConfigMap     = "ConfigMap"
	KindSecret        = "Secret"
)

// apiVersions of supported kinds.
var apiVersions = map[string]string{
	KindNamespace: "v1", KindNode: "v1", KindPod: "v1", KindService: "v1", KindConfigMap: "v1", KindSecret: "v1",
	KindDeployment: "apps/v1", KindReplicaSet: "apps/v1", KindEndpointSlice: "discovery.k8s.io/v1",
}

// ClusterScoped reports whether a kind has no namespace.
func ClusterScoped(kind string) bool { return kind == KindNamespace || kind == KindNode }

// SupportedKind reports whether kind is simulated.
func SupportedKind(kind string) bool { _, ok := apiVersions[kind]; return ok }

// APIVersion returns the apiVersion for a supported kind.
func APIVersion(kind string) string { return apiVersions[kind] }

// SimulatedAnnotation marks every object as simulated in -o yaml/json output.
const SimulatedAnnotation = "kubedojo.dev/simulated"

// OwnerReference links a dependent to its owner.
type OwnerReference struct {
	APIVersion         string `json:"apiVersion"`
	Kind               string `json:"kind"`
	Name               string `json:"name"`
	UID                string `json:"uid"`
	Controller         *bool  `json:"controller,omitempty"`
	BlockOwnerDeletion *bool  `json:"blockOwnerDeletion,omitempty"`
}

// ObjectMeta is the supported subset of metav1.ObjectMeta.
type ObjectMeta struct {
	Name              string            `json:"name"`
	GenerateName      string            `json:"generateName,omitempty"`
	Namespace         string            `json:"namespace,omitempty"`
	UID               string            `json:"uid"`
	ResourceVersion   string            `json:"resourceVersion"`
	Generation        int64             `json:"generation,omitempty"`
	CreationTimestamp time.Time         `json:"creationTimestamp"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	OwnerReferences   []OwnerReference  `json:"ownerReferences,omitempty"`
}

// Object is implemented by every simulated kind.
type Object interface {
	GetKind() string
	Meta() *ObjectMeta
	// DeepCopy returns an independent copy, so controllers never alias stored state.
	DeepCopy() Object
}

// TypeMeta carries apiVersion and kind.
type TypeMeta struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
}

func deepCopy[T any](v T) T {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err) // all simulator types are JSON-safe by construction
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		panic(err)
	}
	return out
}

// ---- Namespace ----

// Namespace is a v1 Namespace.
type Namespace struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta      `json:"metadata"`
	Status   NamespaceStatus `json:"status"`
}

// NamespaceStatus holds the phase.
type NamespaceStatus struct {
	Phase string `json:"phase"`
}

// Meta implements Object.
func (o *Namespace) Meta() *ObjectMeta { return &o.Metadata }

// GetKind returns Namespace, whether or not TypeMeta is filled in.
func (o *Namespace) GetKind() string { return KindNamespace }

// DeepCopy implements Object.
func (o *Namespace) DeepCopy() Object { c := deepCopy(*o); return &c }

// ---- Node ----

// Taint is a node taint.
type Taint struct {
	Key    string `json:"key"`
	Value  string `json:"value,omitempty"`
	Effect string `json:"effect"`
}

// NodeCondition is a node condition.
type NodeCondition struct {
	Type   string `json:"type"`
	Status string `json:"status"`
}

// Node is a v1 Node. Nodes come from fixtures and are never created by learners.
type Node struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta `json:"metadata"`
	Spec     NodeSpec   `json:"spec"`
	Status   NodeStatus `json:"status"`
}

// NodeSpec is the supported node spec.
type NodeSpec struct {
	Taints        []Taint `json:"taints,omitempty"`
	Unschedulable bool    `json:"unschedulable,omitempty"`
}

// NodeAddress is an address of a node.
type NodeAddress struct {
	Type    string `json:"type"`
	Address string `json:"address"`
}

// NodeStatus is the supported node status.
type NodeStatus struct {
	Capacity    map[string]string `json:"capacity"`
	Allocatable map[string]string `json:"allocatable"`
	Conditions  []NodeCondition   `json:"conditions"`
	Addresses   []NodeAddress     `json:"addresses,omitempty"`
	NodeInfo    *NodeSystemInfo   `json:"nodeInfo,omitempty"`
}

// NodeSystemInfo is a small, fixed description of the simulated node software.
type NodeSystemInfo struct {
	KubeletVersion          string `json:"kubeletVersion"`
	ContainerRuntimeVersion string `json:"containerRuntimeVersion"`
	OSImage                 string `json:"osImage"`
	Architecture            string `json:"architecture"`
	OperatingSystem         string `json:"operatingSystem"`
}

// Meta implements Object.
func (o *Node) Meta() *ObjectMeta { return &o.Metadata }

// GetKind returns Node, whether or not TypeMeta is filled in.
func (o *Node) GetKind() string { return KindNode }

// DeepCopy implements Object.
func (o *Node) DeepCopy() Object { c := deepCopy(*o); return &c }

// Ready reports whether the node's Ready condition is True.
func (o *Node) Ready() bool {
	for _, c := range o.Status.Conditions {
		if c.Type == "Ready" {
			return c.Status == "True"
		}
	}
	return false
}

// ---- Pods ----

// EnvVar is a container environment variable.
type EnvVar struct {
	Name      string        `json:"name"`
	Value     string        `json:"value,omitempty"`
	ValueFrom *EnvVarSource `json:"valueFrom,omitempty"`
}

// EnvVarSource references a ConfigMap or Secret key.
type EnvVarSource struct {
	ConfigMapKeyRef *KeySelector `json:"configMapKeyRef,omitempty"`
	SecretKeyRef    *KeySelector `json:"secretKeyRef,omitempty"`
}

// KeySelector selects a key of a ConfigMap or Secret.
type KeySelector struct {
	Name     string `json:"name"`
	Key      string `json:"key"`
	Optional *bool  `json:"optional,omitempty"`
}

// EnvFromSource imports all keys of a ConfigMap or Secret.
type EnvFromSource struct {
	ConfigMapRef *NameRef `json:"configMapRef,omitempty"`
	SecretRef    *NameRef `json:"secretRef,omitempty"`
}

// NameRef references an object by name.
type NameRef struct {
	Name     string `json:"name"`
	Optional *bool  `json:"optional,omitempty"`
}

// ContainerPort is a declared container port.
type ContainerPort struct {
	Name          string `json:"name,omitempty"`
	ContainerPort int32  `json:"containerPort"`
	Protocol      string `json:"protocol,omitempty"`
}

// ResourceRequirements are CPU and memory requests and limits.
type ResourceRequirements struct {
	Requests map[string]string `json:"requests,omitempty"`
	Limits   map[string]string `json:"limits,omitempty"`
}

// Container is the supported container subset.
type Container struct {
	Name      string               `json:"name"`
	Image     string               `json:"image"`
	Command   []string             `json:"command,omitempty"`
	Args      []string             `json:"args,omitempty"`
	Env       []EnvVar             `json:"env,omitempty"`
	EnvFrom   []EnvFromSource      `json:"envFrom,omitempty"`
	Ports     []ContainerPort      `json:"ports,omitempty"`
	Resources ResourceRequirements `json:"resources,omitempty"`
}

// Toleration tolerates a taint.
type Toleration struct {
	Key      string `json:"key,omitempty"`
	Operator string `json:"operator,omitempty"`
	Value    string `json:"value,omitempty"`
	Effect   string `json:"effect,omitempty"`
}

// PodSpec is the supported pod spec.
type PodSpec struct {
	Containers    []Container       `json:"containers"`
	NodeName      string            `json:"nodeName,omitempty"`
	NodeSelector  map[string]string `json:"nodeSelector,omitempty"`
	Tolerations   []Toleration      `json:"tolerations,omitempty"`
	RestartPolicy string            `json:"restartPolicy,omitempty"`
}

// PodCondition is a pod condition.
type PodCondition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	Reason             string    `json:"reason,omitempty"`
	Message            string    `json:"message,omitempty"`
	LastTransitionTime time.Time `json:"lastTransitionTime"`
}

// ContainerState is one of waiting, running or terminated.
type ContainerState struct {
	Waiting    *ContainerStateWaiting    `json:"waiting,omitempty"`
	Running    *ContainerStateRunning    `json:"running,omitempty"`
	Terminated *ContainerStateTerminated `json:"terminated,omitempty"`
}

// ContainerStateWaiting describes a waiting container.
type ContainerStateWaiting struct {
	Reason  string `json:"reason"`
	Message string `json:"message,omitempty"`
}

// ContainerStateRunning describes a running container.
type ContainerStateRunning struct {
	StartedAt time.Time `json:"startedAt"`
}

// ContainerStateTerminated describes a terminated container.
type ContainerStateTerminated struct {
	ExitCode   int32     `json:"exitCode"`
	Reason     string    `json:"reason"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
}

// ContainerStatus is a container's observed status.
type ContainerStatus struct {
	Name         string         `json:"name"`
	Image        string         `json:"image"`
	Ready        bool           `json:"ready"`
	Started      bool           `json:"started"`
	RestartCount int32          `json:"restartCount"`
	State        ContainerState `json:"state"`
	LastState    ContainerState `json:"lastState"`
}

// PodStatus is the observed pod status.
type PodStatus struct {
	Phase             string            `json:"phase,omitempty"`
	Conditions        []PodCondition    `json:"conditions,omitempty"`
	HostIP            string            `json:"hostIP,omitempty"`
	PodIP             string            `json:"podIP,omitempty"`
	StartTime         *time.Time        `json:"startTime,omitempty"`
	ContainerStatuses []ContainerStatus `json:"containerStatuses,omitempty"`
}

// Pod is a v1 Pod.
type Pod struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta `json:"metadata"`
	Spec     PodSpec    `json:"spec"`
	Status   PodStatus  `json:"status"`
}

// Meta implements Object.
func (o *Pod) Meta() *ObjectMeta { return &o.Metadata }

// GetKind returns Pod, whether or not TypeMeta is filled in.
func (o *Pod) GetKind() string { return KindPod }

// DeepCopy implements Object.
func (o *Pod) DeepCopy() Object { c := deepCopy(*o); return &c }

// Ready reports whether the pod's Ready condition is True.
func (o *Pod) Ready() bool {
	for _, c := range o.Status.Conditions {
		if c.Type == "Ready" {
			return c.Status == "True"
		}
	}
	return false
}

// ---- Workloads ----

// LabelSelectorRequirement is a set-based selector term.
type LabelSelectorRequirement struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values,omitempty"`
}

// LabelSelector is metav1.LabelSelector.
type LabelSelector struct {
	MatchLabels      map[string]string          `json:"matchLabels,omitempty"`
	MatchExpressions []LabelSelectorRequirement `json:"matchExpressions,omitempty"`
}

// PodTemplateSpec is a pod template.
type PodTemplateSpec struct {
	Metadata TemplateMeta `json:"metadata"`
	Spec     PodSpec      `json:"spec"`
}

// TemplateMeta is the metadata allowed in a pod template.
type TemplateMeta struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// IntOrString is an int or a string (maxSurge, targetPort).
type IntOrString struct {
	IntVal int32
	StrVal string
	IsStr  bool
}

// MarshalJSON writes an int or a string.
func (v IntOrString) MarshalJSON() ([]byte, error) {
	if v.IsStr {
		return json.Marshal(v.StrVal)
	}
	return json.Marshal(v.IntVal)
}

// UnmarshalJSON reads an int or a string.
func (v *IntOrString) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		v.IsStr = true
		return json.Unmarshal(b, &v.StrVal)
	}
	v.IsStr = false
	return json.Unmarshal(b, &v.IntVal)
}

// RollingUpdate holds rolling update parameters.
type RollingUpdate struct {
	MaxSurge       *IntOrString `json:"maxSurge,omitempty"`
	MaxUnavailable *IntOrString `json:"maxUnavailable,omitempty"`
}

// DeploymentStrategy is RollingUpdate or Recreate.
type DeploymentStrategy struct {
	Type          string         `json:"type,omitempty"`
	RollingUpdate *RollingUpdate `json:"rollingUpdate,omitempty"`
}

// DeploymentSpec is the supported deployment spec.
type DeploymentSpec struct {
	Replicas             *int32             `json:"replicas,omitempty"`
	Selector             LabelSelector      `json:"selector"`
	Template             PodTemplateSpec    `json:"template"`
	Strategy             DeploymentStrategy `json:"strategy,omitempty"`
	RevisionHistoryLimit *int32             `json:"revisionHistoryLimit,omitempty"`
}

// DeploymentCondition is a deployment condition.
type DeploymentCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// DeploymentStatus is the observed deployment status.
type DeploymentStatus struct {
	ObservedGeneration  int64                 `json:"observedGeneration,omitempty"`
	Replicas            int32                 `json:"replicas"`
	UpdatedReplicas     int32                 `json:"updatedReplicas"`
	ReadyReplicas       int32                 `json:"readyReplicas"`
	AvailableReplicas   int32                 `json:"availableReplicas"`
	UnavailableReplicas int32                 `json:"unavailableReplicas,omitempty"`
	Conditions          []DeploymentCondition `json:"conditions,omitempty"`
}

// Deployment is an apps/v1 Deployment.
type Deployment struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta       `json:"metadata"`
	Spec     DeploymentSpec   `json:"spec"`
	Status   DeploymentStatus `json:"status"`
}

// Meta implements Object.
func (o *Deployment) Meta() *ObjectMeta { return &o.Metadata }

// GetKind returns Deployment, whether or not TypeMeta is filled in.
func (o *Deployment) GetKind() string { return KindDeployment }

// DeepCopy implements Object.
func (o *Deployment) DeepCopy() Object { c := deepCopy(*o); return &c }

// DesiredReplicas defaults spec.replicas to 1.
func (o *Deployment) DesiredReplicas() int32 {
	if o.Spec.Replicas == nil {
		return 1
	}
	return *o.Spec.Replicas
}

// ReplicaSetSpec is the supported replica set spec.
type ReplicaSetSpec struct {
	Replicas *int32          `json:"replicas,omitempty"`
	Selector LabelSelector   `json:"selector"`
	Template PodTemplateSpec `json:"template"`
}

// ReplicaSetStatus is the observed replica set status.
type ReplicaSetStatus struct {
	Replicas             int32 `json:"replicas"`
	FullyLabeledReplicas int32 `json:"fullyLabeledReplicas"`
	ReadyReplicas        int32 `json:"readyReplicas"`
	AvailableReplicas    int32 `json:"availableReplicas"`
	ObservedGeneration   int64 `json:"observedGeneration,omitempty"`
}

// ReplicaSet is an apps/v1 ReplicaSet.
type ReplicaSet struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta       `json:"metadata"`
	Spec     ReplicaSetSpec   `json:"spec"`
	Status   ReplicaSetStatus `json:"status"`
}

// Meta implements Object.
func (o *ReplicaSet) Meta() *ObjectMeta { return &o.Metadata }

// GetKind returns ReplicaSet, whether or not TypeMeta is filled in.
func (o *ReplicaSet) GetKind() string { return KindReplicaSet }

// DeepCopy implements Object.
func (o *ReplicaSet) DeepCopy() Object { c := deepCopy(*o); return &c }

// DesiredReplicas defaults spec.replicas to 1.
func (o *ReplicaSet) DesiredReplicas() int32 {
	if o.Spec.Replicas == nil {
		return 1
	}
	return *o.Spec.Replicas
}

// ---- Services ----

// ServicePort is a service port.
type ServicePort struct {
	Name       string      `json:"name,omitempty"`
	Protocol   string      `json:"protocol,omitempty"`
	Port       int32       `json:"port"`
	TargetPort IntOrString `json:"targetPort"`
	NodePort   int32       `json:"nodePort,omitempty"`
}

// ServiceSpec is the supported service spec.
type ServiceSpec struct {
	Type       string            `json:"type,omitempty"`
	Selector   map[string]string `json:"selector,omitempty"`
	Ports      []ServicePort     `json:"ports"`
	ClusterIP  string            `json:"clusterIP,omitempty"`
	ClusterIPs []string          `json:"clusterIPs,omitempty"`
}

// Service is a v1 Service.
type Service struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta    `json:"metadata"`
	Spec     ServiceSpec   `json:"spec"`
	Status   ServiceStatus `json:"status"`
}

// ServiceStatus is the observed service status (no load balancers are provisioned).
type ServiceStatus struct {
	LoadBalancer struct{} `json:"loadBalancer"`
}

// Meta implements Object.
func (o *Service) Meta() *ObjectMeta { return &o.Metadata }

// GetKind returns Service, whether or not TypeMeta is filled in.
func (o *Service) GetKind() string { return KindService }

// DeepCopy implements Object.
func (o *Service) DeepCopy() Object { c := deepCopy(*o); return &c }

// EndpointConditions describe an endpoint's readiness.
type EndpointConditions struct {
	Ready       bool `json:"ready"`
	Serving     bool `json:"serving"`
	Terminating bool `json:"terminating"`
}

// ObjectReference references the Pod behind an endpoint.
type ObjectReference struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	UID       string `json:"uid"`
}

// Endpoint is one endpoint of a slice.
type Endpoint struct {
	Addresses  []string           `json:"addresses"`
	Conditions EndpointConditions `json:"conditions"`
	NodeName   string             `json:"nodeName,omitempty"`
	TargetRef  *ObjectReference   `json:"targetRef,omitempty"`
}

// EndpointPort is a port of a slice.
type EndpointPort struct {
	Name     string `json:"name"`
	Port     int32  `json:"port"`
	Protocol string `json:"protocol"`
}

// EndpointSlice is a discovery.k8s.io/v1 EndpointSlice, managed by the simulator.
type EndpointSlice struct {
	TypeMeta    `json:",inline"`
	Metadata    ObjectMeta     `json:"metadata"`
	AddressType string         `json:"addressType"`
	Endpoints   []Endpoint     `json:"endpoints"`
	Ports       []EndpointPort `json:"ports"`
}

// Meta implements Object.
func (o *EndpointSlice) Meta() *ObjectMeta { return &o.Metadata }

// GetKind returns EndpointSlice, whether or not TypeMeta is filled in.
func (o *EndpointSlice) GetKind() string { return KindEndpointSlice }

// DeepCopy implements Object.
func (o *EndpointSlice) DeepCopy() Object { c := deepCopy(*o); return &c }

// ---- Configuration ----

// ConfigMap is a v1 ConfigMap.
type ConfigMap struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta        `json:"metadata"`
	Data     map[string]string `json:"data,omitempty"`
}

// Meta implements Object.
func (o *ConfigMap) Meta() *ObjectMeta { return &o.Metadata }

// GetKind returns ConfigMap, whether or not TypeMeta is filled in.
func (o *ConfigMap) GetKind() string { return KindConfigMap }

// DeepCopy implements Object.
func (o *ConfigMap) DeepCopy() Object { c := deepCopy(*o); return &c }

// Secret is a v1 Secret of type Opaque. Values are training data only; they are base64-encoded
// in JSON exactly like Kubernetes, which is not encryption (lesson: secrets-awareness).
type Secret struct {
	TypeMeta `json:",inline"`
	Metadata ObjectMeta        `json:"metadata"`
	Type     string            `json:"type"`
	Data     map[string][]byte `json:"data,omitempty"`
}

// Meta implements Object.
func (o *Secret) Meta() *ObjectMeta { return &o.Metadata }

// GetKind returns Secret, whether or not TypeMeta is filled in.
func (o *Secret) GetKind() string { return KindSecret }

// DeepCopy implements Object.
func (o *Secret) DeepCopy() Object { c := deepCopy(*o); return &c }

// ---- Events ----

// Event is a simulated diagnostic event. Events are not resources: they live in their own
// ordered log per lab.
type Event struct {
	Type      string    `json:"type"`
	Reason    string    `json:"reason"`
	Kind      string    `json:"kind"`
	Namespace string    `json:"namespace"`
	Name      string    `json:"name"`
	Message   string    `json:"message"`
	Time      time.Time `json:"time"`
}
