package simulator

import (
	"fmt"
	"slices"
	"time"
)

// The API server's checks for learner writes. Controllers write with Put directly; everything
// a learner asks for goes through Create, Update or Remove, so it is validated like a real
// request and refused with the API server's reasons.

// NotFoundError means the object, or the namespace a new object would live in, doesn't exist.
type NotFoundError struct{ Kind, Name string }

func (e *NotFoundError) Error() string { return fmt.Sprintf("%s %q not found", e.Kind, e.Name) }

// AlreadyExistsError means an object with this kind, namespace and name exists.
type AlreadyExistsError struct{ Kind, Name string }

func (e *AlreadyExistsError) Error() string {
	return fmt.Sprintf("%s %q already exists", e.Kind, e.Name)
}

// ForbiddenError means the cluster refuses the operation, e.g. deleting a system namespace.
type ForbiddenError struct{ Kind, Name, Reason string }

func (e *ForbiddenError) Error() string {
	return fmt.Sprintf("%s %q is forbidden: %s", e.Kind, e.Name, e.Reason)
}

// learnerWritable reports whether learners may create, change or delete objects of kind.
// Nodes come from fixtures; EndpointSlices belong to their controller.
func learnerWritable(kind string) bool {
	return SupportedKind(kind) && kind != KindNode && kind != KindEndpointSlice
}

// Create stores a new object as the API server does for a create request: defaults are applied,
// the object is validated, its namespace must exist, and server-owned metadata is assigned
// here rather than taken from the input.
func (s *State) Create(o Object) error {
	kind := o.GetKind()
	m := o.Meta()
	if !learnerWritable(kind) {
		return &ForbiddenError{Kind: kind, Name: m.Name, Reason: managedReason(kind)}
	}
	if ClusterScoped(kind) {
		m.Namespace = ""
	}
	applyDefaults(o)
	if err := Validate(o); err != nil {
		return err
	}
	if !ClusterScoped(kind) && !s.Has(KindNamespace, "", m.Namespace) {
		return &NotFoundError{Kind: KindNamespace, Name: m.Namespace}
	}
	if s.Has(kind, m.Namespace, m.Name) {
		return &AlreadyExistsError{Kind: kind, Name: m.Name}
	}
	if err := s.checkNodePorts(o); err != nil {
		return err
	}
	m.UID, m.ResourceVersion, m.Generation, m.CreationTimestamp = "", "", 0, time.Time{}
	switch v := o.(type) {
	case *Namespace:
		v.Status.Phase = "Active"
		if v.Metadata.Labels == nil {
			v.Metadata.Labels = map[string]string{}
		}
		v.Metadata.Labels["kubernetes.io/metadata.name"] = v.Metadata.Name
	case *Pod:
		v.Status = PodStatus{Phase: "Pending"}
	case *Deployment:
		v.Status = DeploymentStatus{}
	case *ReplicaSet:
		v.Status = ReplicaSetStatus{}
	}
	return s.Put(o)
}

// Update replaces an existing object (a learner's update request): the result is validated,
// immutable fields must not change, and status stays the cluster's.
func (s *State) Update(o Object) error {
	kind := o.GetKind()
	m := o.Meta()
	if !learnerWritable(kind) && kind != KindNode {
		return &ForbiddenError{Kind: kind, Name: m.Name, Reason: managedReason(kind)}
	}
	old := s.Get(kind, m.Namespace, m.Name)
	if old == nil {
		return &NotFoundError{Kind: kind, Name: m.Name}
	}
	if kind == KindNode {
		// Learners may only label and annotate Nodes.
		n := old.(*Node)
		n.Metadata.Labels, n.Metadata.Annotations = m.Labels, m.Annotations
		return s.Put(n)
	}
	applyDefaults(o)
	if err := Validate(o); err != nil {
		return err
	}
	if err := ValidateUpdate(old, o); err != nil {
		return err
	}
	if err := s.checkNodePorts(o); err != nil {
		return err
	}
	keepStatus(old, o)
	return s.Put(o)
}

// keepStatus copies the stored status into an update, as the API server ignores status in
// spec updates.
func keepStatus(old, updated Object) {
	switch o := old.(type) {
	case *Pod:
		u := updated.(*Pod)
		u.Status = o.Status
		u.Spec.NodeName = o.Spec.NodeName
	case *Deployment:
		updated.(*Deployment).Status = o.Status
	case *ReplicaSet:
		updated.(*ReplicaSet).Status = o.Status
	case *Service:
		u := updated.(*Service)
		u.Status = o.Status
		if u.Spec.ClusterIP == "" {
			u.Spec.ClusterIP, u.Spec.ClusterIPs = o.Spec.ClusterIP, o.Spec.ClusterIPs
		}
	case *Namespace:
		updated.(*Namespace).Status = o.Status
	}
}

// checkNodePorts refuses a Service that asks for a NodePort another Service already holds.
func (s *State) checkNodePorts(o Object) error {
	svc, ok := o.(*Service)
	if !ok {
		return nil
	}
	used := map[int32]bool{}
	for _, other := range s.List(KindService, "*") {
		om := other.Meta()
		if om.Namespace == svc.Metadata.Namespace && om.Name == svc.Metadata.Name {
			continue
		}
		for _, p := range other.(*Service).Spec.Ports {
			used[p.NodePort] = true
		}
	}
	var reasons []string
	for i, p := range svc.Spec.Ports {
		if p.NodePort != 0 && used[p.NodePort] {
			reasons = append(reasons, fmt.Sprintf("spec.ports[%d].nodePort: Invalid value: %d: provided port is already allocated", i, p.NodePort))
		}
	}
	if len(reasons) > 0 {
		return &ValidationError{Kind: KindService, Name: svc.Metadata.Name, Reasons: reasons}
	}
	return nil
}

func managedReason(kind string) string {
	switch kind {
	case KindNode:
		return "nodes are part of the simulated cluster and can't be created or deleted (labels and annotations can change)"
	case KindEndpointSlice:
		return "EndpointSlices are managed by the EndpointSlice controller; change the Service or its Pods instead"
	}
	return kind + " is not part of the KubeDojo simulator"
}

// Remove deletes an object as a delete request does. Dependents are deleted by the garbage
// collector on the next reconcile, unless orphan is set, in which case their controller
// reference is removed so they survive.
func (s *State) Remove(kind, namespace, name string, orphan bool) error {
	if ClusterScoped(kind) {
		namespace = ""
	}
	o := s.Get(kind, namespace, name)
	if o == nil {
		return &NotFoundError{Kind: kind, Name: name}
	}
	switch {
	case !learnerWritable(kind):
		return &ForbiddenError{Kind: kind, Name: name, Reason: managedReason(kind)}
	case kind == KindNamespace && slices.Contains(SystemNamespaces, name):
		return &ForbiddenError{Kind: kind, Name: name, Reason: "this namespace may not be deleted"}
	case kind == KindService && namespace == "default" && name == "kubernetes":
		return &ForbiddenError{Kind: kind, Name: name, Reason: "this Service is managed by the API server (KubeDojo keeps it in place)"}
	}
	if orphan {
		uid := o.Meta().UID
		for _, dep := range s.All() {
			refs := dep.Meta().OwnerReferences
			kept := refs[:0:0]
			for _, ref := range refs {
				if ref.UID != uid {
					kept = append(kept, ref)
				}
			}
			if len(kept) == len(refs) {
				continue
			}
			if len(kept) == 0 {
				kept = nil
			}
			dep.Meta().OwnerReferences = kept
			if err := s.Put(dep); err != nil {
				return err
			}
		}
	}
	if p, ok := o.(*Pod); ok {
		for _, c := range p.Status.ContainerStatuses {
			if c.State.Running != nil {
				s.Record("Normal", "Killing", p, "Stopping container "+c.Name)
			}
		}
	}
	s.Delete(kind, namespace, name)
	return nil
}
