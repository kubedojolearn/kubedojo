package simulator

import (
	"errors"
	"fmt"
	"net/netip"
	"sort"
)

// MaxReconcilePasses bounds the fixed-point loop. A rolling update of the largest allowed
// Deployment (50 replicas, maxSurge 1, maxUnavailable 0) needs about 100 passes; anything that
// still changes after this many passes is a simulator bug.
const MaxReconcilePasses = 256

// ErrNoConvergence means controllers kept changing state: a simulator bug, never learner error.
var ErrNoConvergence = errors.New("simulator did not converge")

// Result reports what a reconcile did.
type Result struct {
	Passes int
}

// controllers run in this order on every pass (ADR-0006).
var controllers = []struct {
	name string
	run  func(*State) error
}{
	{"namespace", reconcileNamespaces},
	{"node", reconcileNodes},
	{"deployment", reconcileDeployments},
	{"replicaset", reconcileReplicaSets},
	{"scheduler", schedulePods},
	{"kubelet", runKubelets},
	{"service", allocateServiceAddresses},
	{"endpointslice", reconcileEndpointSlices},
	{"garbage-collector", collectGarbage},
}

// Reconcile runs controllers until a full pass changes nothing.
func Reconcile(s *State) (Result, error) {
	for pass := 1; pass <= MaxReconcilePasses; pass++ {
		before := s.Revision
		deletedBefore := len(s.deleted)
		for _, c := range controllers {
			if err := c.run(s); err != nil {
				return Result{Passes: pass}, fmt.Errorf("%s controller: %w", c.name, err)
			}
		}
		if s.Revision == before && len(s.deleted) == deletedBefore {
			return Result{Passes: pass}, nil
		}
	}
	return Result{Passes: MaxReconcilePasses}, ErrNoConvergence
}

// reconcileNamespaces deletes namespaced objects whose namespace no longer exists (namespace
// deletion is immediate in the simulator; there is no Terminating phase).
func reconcileNamespaces(s *State) error {
	for k := range s.objects {
		if k.Namespace != "" && !s.Has(KindNamespace, "", k.Namespace) {
			s.Delete(k.Kind, k.Namespace, k.Name)
		}
	}
	return nil
}

// reconcileNodes gives each node a stable InternalIP (192.168.49.2, .3, … in name order).
func reconcileNodes(s *State) error {
	for i, o := range s.List(KindNode, "*") {
		n := o.(*Node)
		want := fmt.Sprintf("192.168.49.%d", i+2)
		has := false
		for _, a := range n.Status.Addresses {
			if a.Type == "InternalIP" {
				has = true
			}
		}
		if !has {
			n.Status.Addresses = append([]NodeAddress{{Type: "InternalIP", Address: want}}, n.Status.Addresses...)
			if err := s.Put(n); err != nil {
				return err
			}
		}
	}
	return nil
}

// nodeIndex returns a node's position in name order (used for Pod CIDRs).
func nodeIndex(s *State, name string) int {
	for i, o := range s.List(KindNode, "*") {
		if o.Meta().Name == name {
			return i
		}
	}
	return -1
}

func nodeInternalIP(n *Node) string {
	for _, a := range n.Status.Addresses {
		if a.Type == "InternalIP" {
			return a.Address
		}
	}
	return ""
}

// allocateServiceAddresses assigns ClusterIPs (lowest free in 10.96.0.0/16 from .10) and
// NodePorts (deterministic picks in 30000-32767) to Services that lack them.
func allocateServiceAddresses(s *State) error {
	usedIPs := map[string]bool{}
	usedPorts := map[int32]bool{}
	services := s.List(KindService, "*")
	for _, o := range services {
		svc := o.(*Service)
		if svc.Spec.ClusterIP != "" {
			usedIPs[svc.Spec.ClusterIP] = true
		}
		for _, p := range svc.Spec.Ports {
			if p.NodePort != 0 {
				usedPorts[p.NodePort] = true
			}
		}
	}
	for _, o := range services {
		svc := o.(*Service)
		changed := false
		if svc.Spec.Type == "" {
			svc.Spec.Type = "ClusterIP"
			changed = true
		}
		if svc.Spec.ClusterIP == "" {
			ip, err := nextClusterIP(usedIPs)
			if err != nil {
				return err
			}
			usedIPs[ip] = true
			svc.Spec.ClusterIP, svc.Spec.ClusterIPs = ip, []string{ip}
			changed = true
		}
		for i := range svc.Spec.Ports {
			p := &svc.Spec.Ports[i]
			if p.Protocol == "" {
				p.Protocol, changed = "TCP", true
			}
			if !p.TargetPort.IsStr && p.TargetPort.IntVal == 0 {
				p.TargetPort, changed = IntOrString{IntVal: p.Port}, true
			}
			needsNodePort := svc.Spec.Type == "NodePort" || svc.Spec.Type == "LoadBalancer"
			if needsNodePort && p.NodePort == 0 {
				for {
					candidate := int32(30000 + s.Gen.next()%2768)
					if !usedPorts[candidate] {
						p.NodePort, usedPorts[candidate] = candidate, true
						break
					}
				}
				changed = true
			}
			if !needsNodePort && p.NodePort != 0 {
				delete(usedPorts, p.NodePort)
				p.NodePort, changed = 0, true
			}
		}
		if changed {
			if err := s.Put(svc); err != nil {
				return err
			}
		}
	}
	return nil
}

func nextClusterIP(used map[string]bool) (string, error) {
	addr := netip.MustParseAddr("10.96.0.10")
	for i := 0; i < 65000; i++ {
		if !used[addr.String()] {
			return addr.String(), nil
		}
		addr = addr.Next()
	}
	return "", fmt.Errorf("%w: no free ClusterIPs", ErrLimit)
}

// collectGarbage deletes objects whose controlling owner no longer exists (cascade deletion).
func collectGarbage(s *State) error {
	uids := map[string]bool{}
	for _, o := range s.objects {
		uids[o.Meta().UID] = true
	}
	var orphans []Key
	for k, o := range s.objects {
		for _, ref := range o.Meta().OwnerReferences {
			if ref.Controller != nil && *ref.Controller && !uids[ref.UID] {
				orphans = append(orphans, k)
			}
		}
	}
	sort.Slice(orphans, func(i, j int) bool { return orphans[i].String() < orphans[j].String() })
	for _, k := range orphans {
		s.Delete(k.Kind, k.Namespace, k.Name)
	}
	return nil
}

func controllerRef(owner Object) OwnerReference {
	t := true
	m := owner.Meta()
	return OwnerReference{APIVersion: APIVersion(owner.GetKind()), Kind: owner.GetKind(), Name: m.Name, UID: m.UID, Controller: &t, BlockOwnerDeletion: &t}
}

// ControllerOf returns the UID of an object's controlling owner, or "".
func ControllerOf(o Object) string {
	for _, ref := range o.Meta().OwnerReferences {
		if ref.Controller != nil && *ref.Controller {
			return ref.UID
		}
	}
	return ""
}
