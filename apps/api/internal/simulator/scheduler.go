package simulator

import (
	"fmt"
	"sort"
	"strings"
)

type podRequests struct{ cpu, memory int64 }

func requestsOf(spec PodSpec) podRequests {
	var r podRequests
	for _, c := range spec.Containers {
		if q, ok := c.Resources.Requests["cpu"]; ok {
			v, _ := ParseCPU(q)
			r.cpu += v
		} else if q, ok := c.Resources.Limits["cpu"]; ok { // requests default to limits
			v, _ := ParseCPU(q)
			r.cpu += v
		}
		if q, ok := c.Resources.Requests["memory"]; ok {
			v, _ := ParseMemory(q)
			r.memory += v
		} else if q, ok := c.Resources.Limits["memory"]; ok {
			v, _ := ParseMemory(q)
			r.memory += v
		}
	}
	return r
}

type nodeCapacity struct {
	cpu, memory, pods int64
}

func allocatable(n *Node) nodeCapacity {
	src := n.Status.Allocatable
	if len(src) == 0 {
		src = n.Status.Capacity
	}
	var c nodeCapacity
	c.cpu, _ = ParseCPU(src["cpu"])
	c.memory, _ = ParseMemory(src["memory"])
	c.pods, _ = ParseCount(src["pods"])
	return c
}

func tolerates(tols []Toleration, t Taint) bool {
	for _, tol := range tols {
		if tol.Effect != "" && tol.Effect != t.Effect {
			continue
		}
		if tol.Operator == "Exists" && (tol.Key == "" || tol.Key == t.Key) {
			return true
		}
		if (tol.Operator == "" || tol.Operator == "Equal") && tol.Key == t.Key && tol.Value == t.Value {
			return true
		}
	}
	return false
}

type nodeUsage struct{ cpu, memory, pods int64 }

// usage sums the requests and counts the Pods bound to each node.
func usage(s *State) map[string]nodeUsage {
	out := map[string]nodeUsage{}
	for _, o := range s.List(KindPod, "*") {
		p := o.(*Pod)
		if p.Spec.NodeName == "" {
			continue
		}
		r := requestsOf(p.Spec)
		u := out[p.Spec.NodeName]
		u.cpu += r.cpu
		u.memory += r.memory
		u.pods++
		out[p.Spec.NodeName] = u
	}
	return out
}

// schedulePods binds pending Pods to nodes (simulator-compatibility.md §2, Scheduler). Filters
// run in kube-scheduler's default plugin order: unschedulable, taints, node selector, resources.
func schedulePods(s *State) error {
	nodes := s.List(KindNode, "*")
	for _, o := range s.List(KindPod, "*") {
		p := o.(*Pod)
		if p.Spec.NodeName != "" {
			continue
		}
		used := usage(s) // recompute per Pod: earlier bindings in this pass consume capacity
		req := requestsOf(p.Spec)
		reasons := map[string]int{}
		type candidate struct {
			name  string
			score float64
		}
		var fits []candidate
		for _, no := range nodes {
			n := no.(*Node)
			name := n.Metadata.Name
			if !n.Ready() {
				reasons["node(s) were not ready"]++
				continue
			}
			if n.Spec.Unschedulable {
				reasons["node(s) were unschedulable"]++
				continue
			}
			untolerated := ""
			for _, t := range n.Spec.Taints {
				if (t.Effect == "NoSchedule" || t.Effect == "NoExecute") && !tolerates(p.Spec.Tolerations, t) {
					untolerated = fmt.Sprintf("node(s) had untolerated taint {%s: %s}", t.Key, t.Value)
					break
				}
			}
			if untolerated != "" {
				reasons[untolerated]++
				continue
			}
			if !SelectorFromLabels(p.Spec.NodeSelector).Matches(n.Metadata.Labels) {
				reasons["node(s) didn't match Pod's node affinity/selector"]++
				continue
			}
			alloc := allocatable(n)
			u := used[name]
			insufficient := false
			if req.cpu > 0 && u.cpu+req.cpu > alloc.cpu {
				reasons["Insufficient cpu"]++
				insufficient = true
			}
			if req.memory > 0 && u.memory+req.memory > alloc.memory {
				reasons["Insufficient memory"]++
				insufficient = true
			}
			if alloc.pods > 0 && u.pods+1 > alloc.pods {
				reasons["Too many pods"]++
				insufficient = true
			}
			if insufficient {
				continue
			}
			score := 0.0
			if alloc.cpu > 0 {
				score = float64(u.cpu+req.cpu) / float64(alloc.cpu)
			}
			fits = append(fits, candidate{name: name, score: score})
		}
		if len(fits) == 0 {
			if err := markUnschedulable(s, p, len(nodes), reasons); err != nil {
				return err
			}
			continue
		}
		sort.Slice(fits, func(i, j int) bool {
			if fits[i].score != fits[j].score {
				return fits[i].score < fits[j].score
			}
			return fits[i].name < fits[j].name
		})
		p.Spec.NodeName = fits[0].name
		setPodCondition(p, PodCondition{Type: "PodScheduled", Status: "True", LastTransitionTime: s.Now})
		if err := s.Put(p); err != nil {
			return err
		}
		s.Record("Normal", "Scheduled", p, fmt.Sprintf("Successfully assigned %s/%s to %s", p.Metadata.Namespace, p.Metadata.Name, fits[0].name))
	}
	return nil
}

func markUnschedulable(s *State, p *Pod, nodeCount int, reasons map[string]int) error {
	// kube-scheduler's FitError sorts the "<count> <reason>" strings.
	parts := make([]string, 0, len(reasons))
	for r, n := range reasons {
		parts = append(parts, fmt.Sprintf("%d %s", n, r))
	}
	sort.Strings(parts)
	msg := fmt.Sprintf("0/%d nodes are available: %s.", nodeCount, strings.Join(parts, ", "))
	if nodeCount == 0 {
		msg = "0/0 nodes are available: no nodes available to schedule pods."
	}
	for _, c := range p.Status.Conditions {
		if c.Type == "PodScheduled" && c.Message == msg {
			return nil // unchanged: don't repeat the event
		}
	}
	setPodCondition(p, PodCondition{Type: "PodScheduled", Status: "False", Reason: "Unschedulable", Message: msg, LastTransitionTime: s.Now})
	p.Status.Phase = "Pending"
	if err := s.Put(p); err != nil {
		return err
	}
	s.Record("Warning", "FailedScheduling", p, msg)
	return nil
}

func setPodCondition(p *Pod, c PodCondition) {
	for i, existing := range p.Status.Conditions {
		if existing.Type == c.Type {
			if existing.Status == c.Status {
				c.LastTransitionTime = existing.LastTransitionTime
			}
			p.Status.Conditions[i] = c
			return
		}
	}
	p.Status.Conditions = append(p.Status.Conditions, c)
}
