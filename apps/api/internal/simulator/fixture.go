package simulator

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// Fixture is a lab's starting cluster (packages/content-schema/schemas/fixture.schema.json).
type Fixture struct {
	SchemaVersion    int        `json:"schemaVersion"`
	SimulatorVersion string     `json:"simulatorVersion"`
	Description      string     `json:"description,omitempty"`
	Epoch            *time.Time `json:"epoch,omitempty"`
	Kubeconfig       *struct {
		Namespace string `json:"namespace,omitempty"`
	} `json:"kubeconfig,omitempty"`
	ImageCatalog []ImageProfile    `json:"imageCatalog,omitempty"`
	Resources    []json.RawMessage `json:"resources"`
}

// DefaultEpoch is the logical start time of labs whose fixture sets none.
var DefaultEpoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// SystemNamespaces exist in every lab and cannot be deleted.
var SystemNamespaces = []string{"default", "kube-node-lease", "kube-public", "kube-system"}

// Load builds the reconciled starting state of a lab from its fixture and seed.
func Load(f Fixture, seed uint64) (*State, error) {
	if f.SimulatorVersion != Version {
		return nil, fmt.Errorf("fixture targets simulator %q; this is %q", f.SimulatorVersion, Version)
	}
	epoch := DefaultEpoch
	if f.Epoch != nil {
		epoch = f.Epoch.UTC()
	}
	s := NewState(epoch, seed, NewImageCatalog(f.ImageCatalog))
	if f.Kubeconfig != nil && f.Kubeconfig.Namespace != "" {
		s.CurrentNamespace = f.Kubeconfig.Namespace
	}

	// System namespaces are as old as the cluster.
	clusterAge := epoch.Add(-30 * 24 * time.Hour)
	for _, ns := range SystemNamespaces {
		n := &Namespace{Metadata: ObjectMeta{Name: ns, CreationTimestamp: clusterAge, Labels: map[string]string{"kubernetes.io/metadata.name": ns}}}
		n.Status.Phase = "Active"
		if err := s.Put(n); err != nil {
			return nil, err
		}
	}

	// Every cluster has the API server's Service in the default namespace.
	s.Now = clusterAge
	api := &Service{
		Metadata: ObjectMeta{Name: "kubernetes", Namespace: "default", CreationTimestamp: clusterAge,
			Labels: map[string]string{"component": "apiserver", "provider": "kubernetes"}},
		Spec: ServiceSpec{Type: "ClusterIP", ClusterIP: "10.96.0.1", ClusterIPs: []string{"10.96.0.1"},
			Ports: []ServicePort{{Name: "https", Protocol: "TCP", Port: 443, TargetPort: IntOrString{IntVal: 6443}}}},
	}
	if err := s.Put(api); err != nil {
		return nil, err
	}

	// Decode and validate everything first, then replay the fixture's history: objects are
	// created in age order, reconciling at each creation time, so controller-created Pods and
	// events are as old as their owners. Finally the clock moves to the lab's start.
	type entry struct {
		obj   Object
		index int
	}
	var entries []entry
	for i, raw := range f.Resources {
		var extra struct {
			AgeSeconds int64 `json:"ageSeconds"`
		}
		_ = json.Unmarshal(raw, &extra)
		var generic map[string]json.RawMessage
		if err := json.Unmarshal(raw, &generic); err != nil {
			return nil, fmt.Errorf("fixture resource %d: %w", i, err)
		}
		delete(generic, "ageSeconds")
		clean, _ := json.Marshal(generic)

		o, err := DecodeInput(clean)
		if err != nil {
			return nil, fmt.Errorf("fixture resource %d: %w", i, err)
		}
		m := o.Meta()
		m.CreationTimestamp = epoch.Add(-time.Duration(extra.AgeSeconds) * time.Second)
		if !ClusterScoped(o.GetKind()) && m.Namespace == "" {
			m.Namespace = "default"
		}
		switch v := o.(type) {
		case *Namespace:
			v.Status.Phase = "Active"
			if v.Metadata.Labels == nil {
				v.Metadata.Labels = map[string]string{}
			}
			v.Metadata.Labels["kubernetes.io/metadata.name"] = v.Metadata.Name
		case *Node:
			prepareNode(v)
		case *EndpointSlice, *ReplicaSet:
			return nil, fmt.Errorf("fixture resource %d: %s objects are created by controllers, not fixtures", i, o.GetKind())
		}
		if err := Validate(o); err != nil {
			return nil, fmt.Errorf("fixture resource %d: %w", i, err)
		}
		entries = append(entries, entry{obj: o, index: i})
	}
	sort.SliceStable(entries, func(a, b int) bool {
		ta, tb := entries[a].obj.Meta().CreationTimestamp, entries[b].obj.Meta().CreationTimestamp
		if !ta.Equal(tb) {
			return ta.Before(tb)
		}
		// Nodes first at equal age, so Pods created alongside them can be scheduled.
		return entries[a].obj.GetKind() == KindNode && entries[b].obj.GetKind() != KindNode
	})
	for _, e := range entries {
		o, m := e.obj, e.obj.Meta()
		s.Now = m.CreationTimestamp
		if !ClusterScoped(o.GetKind()) && !s.Has(KindNamespace, "", m.Namespace) {
			ns := &Namespace{Metadata: ObjectMeta{Name: m.Namespace, CreationTimestamp: m.CreationTimestamp, Labels: map[string]string{"kubernetes.io/metadata.name": m.Namespace}}}
			ns.Status.Phase = "Active"
			if err := s.Put(ns); err != nil {
				return nil, err
			}
		}
		if s.Has(o.GetKind(), m.Namespace, m.Name) {
			return nil, fmt.Errorf("fixture resource %d: duplicate %s", e.index, KeyOf(o))
		}
		if err := s.Put(o); err != nil {
			return nil, err
		}
		if _, err := Reconcile(s); err != nil {
			return nil, err
		}
	}
	s.Now = epoch
	if _, err := Reconcile(s); err != nil {
		return nil, err
	}
	return s, nil
}

func prepareNode(n *Node) {
	if n.Status.NodeInfo == nil {
		n.Status.NodeInfo = &NodeSystemInfo{
			KubeletVersion: "v1.37.1", ContainerRuntimeVersion: "containerd://2.1.4 (simulated)",
			OSImage: "KubeDojo simulated node", Architecture: "amd64", OperatingSystem: "linux",
		}
	}
	if len(n.Status.Addresses) == 0 {
		n.Status.Addresses = []NodeAddress{{Type: "Hostname", Address: n.Metadata.Name}}
	}
	if n.Metadata.Labels == nil {
		n.Metadata.Labels = map[string]string{}
	}
	if _, ok := n.Metadata.Labels["kubernetes.io/hostname"]; !ok {
		n.Metadata.Labels["kubernetes.io/hostname"] = n.Metadata.Name
	}
	n.Metadata.Labels["kubernetes.io/os"] = "linux"
}
