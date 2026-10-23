package kubectl

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// Columns follow kubectl v1.37's built-in printers for each kind.

func podStatus(p *simulator.Pod) string {
	reason := p.Status.Phase
	if reason == "" {
		reason = "Pending"
	}
	for _, cs := range p.Status.ContainerStatuses {
		switch {
		case cs.State.Waiting != nil && cs.State.Waiting.Reason != "":
			reason = cs.State.Waiting.Reason
		case cs.State.Terminated != nil && cs.State.Terminated.Reason != "":
			reason = cs.State.Terminated.Reason
		}
	}
	return reason
}

func podReady(p *simulator.Pod) (ready, total int) {
	total = len(p.Spec.Containers)
	for _, cs := range p.Status.ContainerStatuses {
		if cs.Ready {
			ready++
		}
	}
	return ready, total
}

// podRestarts is the RESTARTS column: "3 (25s ago)" when a container has restarted.
func podRestarts(p *simulator.Pod, now time.Time) string {
	var restarts int32
	var last time.Time
	for _, cs := range p.Status.ContainerStatuses {
		restarts += cs.RestartCount
		if t := cs.LastState.Terminated; t != nil && t.FinishedAt.After(last) {
			last = t.FinishedAt
		}
	}
	if restarts > 0 && !last.IsZero() {
		return fmt.Sprintf("%d (%s ago)", restarts, humanDuration(now.Sub(last)))
	}
	return strconv.Itoa(int(restarts))
}

func containerNamesImages(spec simulator.PodSpec) (string, string) {
	var names, images []string
	for _, c := range spec.Containers {
		names = append(names, c.Name)
		images = append(images, c.Image)
	}
	return strings.Join(names, ","), strings.Join(images, ",")
}

func selectorString(ls simulator.LabelSelector) string {
	sel, err := simulator.SelectorFromLabelSelector(ls)
	if err != nil || sel.Empty() {
		return "<none>"
	}
	return sel.String()
}

func servicePorts(svc *simulator.Service) string {
	if len(svc.Spec.Ports) == 0 {
		return "<none>"
	}
	parts := make([]string, 0, len(svc.Spec.Ports))
	for _, p := range svc.Spec.Ports {
		proto := p.Protocol
		if proto == "" {
			proto = "TCP"
		}
		if p.NodePort != 0 {
			parts = append(parts, fmt.Sprintf("%d:%d/%s", p.Port, p.NodePort, proto))
		} else {
			parts = append(parts, fmt.Sprintf("%d/%s", p.Port, proto))
		}
	}
	return strings.Join(parts, ",")
}

func externalIP(svc *simulator.Service) string {
	if svc.Spec.Type == "LoadBalancer" {
		return "<pending>"
	}
	return "<none>"
}

func nodeStatus(n *simulator.Node) string {
	s := "NotReady"
	if n.Ready() {
		s = "Ready"
	}
	if n.Spec.Unschedulable {
		s += ",SchedulingDisabled"
	}
	return s
}

func nodeRoles(n *simulator.Node) string {
	var roles []string
	for k := range n.Metadata.Labels {
		if r, ok := strings.CutPrefix(k, "node-role.kubernetes.io/"); ok && r != "" {
			roles = append(roles, r)
		}
	}
	sort.Strings(roles)
	if len(roles) == 0 {
		return "<none>"
	}
	return strings.Join(roles, ",")
}

func nodeAddress(n *simulator.Node, typ string) string {
	for _, a := range n.Status.Addresses {
		if a.Type == typ {
			return a.Address
		}
	}
	return "<none>"
}

// formatAddresses lists up to three entries, as kubectl does for endpoints.
func formatAddresses(addrs []string) string {
	if len(addrs) == 0 {
		return "<none>"
	}
	if len(addrs) > 3 {
		return strings.Join(addrs[:3], ",") + fmt.Sprintf(" + %d more...", len(addrs)-3)
	}
	return strings.Join(addrs, ",")
}

func sliceEndpoints(s *simulator.EndpointSlice) string {
	var addrs []string
	for _, e := range s.Endpoints {
		addrs = append(addrs, e.Addresses...)
	}
	if len(addrs) == 0 {
		return "<unset>"
	}
	return formatAddresses(addrs)
}

func slicePorts(s *simulator.EndpointSlice) string {
	if len(s.Ports) == 0 {
		return "<unset>"
	}
	var ports []string
	for _, p := range s.Ports {
		ports = append(ports, strconv.Itoa(int(p.Port)))
	}
	return strings.Join(ports, ",")
}

func eventObject(e simulator.Event) string {
	return strings.ToLower(e.Kind) + "/" + e.Name
}

// columns returns the header and row for one item of a kind. Shared NAMESPACE and LABELS
// columns are added by the caller.
func columns(it item, wide bool, now time.Time) (header, row []string) {
	ageCol := age(now, it.created)
	switch o := it.obj.(type) {
	case *simulator.Pod:
		ready, total := podReady(o)
		header = []string{"NAME", "READY", "STATUS", "RESTARTS", "AGE"}
		row = []string{it.display, fmt.Sprintf("%d/%d", ready, total), podStatus(o), podRestarts(o, now), ageCol}
		if wide {
			header = append(header, "IP", "NODE", "NOMINATED NODE", "READINESS GATES")
			row = append(row, orNone(o.Status.PodIP), orNone(o.Spec.NodeName), "<none>", "<none>")
		}
	case *simulator.Deployment:
		names, images := containerNamesImages(o.Spec.Template.Spec)
		header = []string{"NAME", "READY", "UP-TO-DATE", "AVAILABLE", "AGE"}
		row = []string{it.display, fmt.Sprintf("%d/%d", o.Status.ReadyReplicas, o.DesiredReplicas()),
			strconv.Itoa(int(o.Status.UpdatedReplicas)), strconv.Itoa(int(o.Status.AvailableReplicas)), ageCol}
		if wide {
			header = append(header, "CONTAINERS", "IMAGES", "SELECTOR")
			row = append(row, names, images, selectorString(o.Spec.Selector))
		}
	case *simulator.ReplicaSet:
		names, images := containerNamesImages(o.Spec.Template.Spec)
		header = []string{"NAME", "DESIRED", "CURRENT", "READY", "AGE"}
		row = []string{it.display, strconv.Itoa(int(o.DesiredReplicas())), strconv.Itoa(int(o.Status.Replicas)),
			strconv.Itoa(int(o.Status.ReadyReplicas)), ageCol}
		if wide {
			header = append(header, "CONTAINERS", "IMAGES", "SELECTOR")
			row = append(row, names, images, selectorString(o.Spec.Selector))
		}
	case *simulator.Service:
		typ := o.Spec.Type
		if typ == "" {
			typ = "ClusterIP"
		}
		header = []string{"NAME", "TYPE", "CLUSTER-IP", "EXTERNAL-IP", "PORT(S)", "AGE"}
		row = []string{it.display, typ, orNone(o.Spec.ClusterIP), externalIP(o), servicePorts(o), ageCol}
		if wide {
			header = append(header, "SELECTOR")
			row = append(row, labelString(o.Spec.Selector))
		}
	case *simulator.Namespace:
		phase := o.Status.Phase
		if phase == "" {
			phase = "Active"
		}
		header = []string{"NAME", "STATUS", "AGE"}
		row = []string{it.display, phase, ageCol}
	case *simulator.Node:
		version, osImage, runtime := "", "<unknown>", "<unknown>"
		if info := o.Status.NodeInfo; info != nil {
			version, osImage, runtime = info.KubeletVersion, info.OSImage, info.ContainerRuntimeVersion
		}
		header = []string{"NAME", "STATUS", "ROLES", "AGE", "VERSION"}
		row = []string{it.display, nodeStatus(o), nodeRoles(o), ageCol, version}
		if wide {
			header = append(header, "INTERNAL-IP", "EXTERNAL-IP", "OS-IMAGE", "KERNEL-VERSION", "CONTAINER-RUNTIME")
			row = append(row, nodeAddress(o, "InternalIP"), nodeAddress(o, "ExternalIP"), osImage, "<unknown>", runtime)
		}
	case *simulator.ConfigMap:
		header = []string{"NAME", "DATA", "AGE"}
		row = []string{it.display, strconv.Itoa(len(o.Data)), ageCol}
	case *simulator.Secret:
		header = []string{"NAME", "TYPE", "DATA", "AGE"}
		row = []string{it.display, o.Type, strconv.Itoa(len(o.Data)), ageCol}
	case *simulator.EndpointSlice:
		header = []string{"NAME", "ADDRESSTYPE", "PORTS", "ENDPOINTS", "AGE"}
		row = []string{it.display, o.AddressType, slicePorts(o), sliceEndpoints(o), ageCol}
	default:
		switch {
		case it.event != nil:
			e := *it.event
			header = []string{"LAST SEEN", "TYPE", "REASON", "OBJECT", "MESSAGE"}
			row = []string{ageCol, e.Type, e.Reason, eventObject(e), e.Message}
		case it.endpoints != nil:
			header = []string{"NAME", "ENDPOINTS", "AGE"}
			row = []string{it.display, formatAddresses(it.endpoints), ageCol}
		}
	}
	return header, row
}
