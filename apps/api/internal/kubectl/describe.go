package kubectl

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// describer reproduces kubectl's describe layout: lines with tab-separated cells at an indent
// level, aligned by one tabwriter (min width 0, tab width 8, padding 2) over the whole output.
type describer struct {
	buf bytes.Buffer
	now time.Time
}

func (d *describer) line(level int, format string, args ...any) {
	d.buf.WriteString(strings.Repeat("  ", level))
	fmt.Fprintf(&d.buf, format, args...)
	d.buf.WriteString("\n")
}

func (d *describer) String() string {
	var out bytes.Buffer
	w := tabwriter.NewWriter(&out, 0, 8, 2, ' ', 0)
	_, _ = w.Write(d.buf.Bytes())
	_ = w.Flush()
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n") + "\n"
}

const describeTime = "Mon, 02 Jan 2006 15:04:05 -0700"

func (d *describer) multiline(level int, title string, m map[string]string, sep string) {
	if len(m) == 0 {
		d.line(level, "%s:\t<none>", title)
		return
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		if i == 0 {
			d.line(level, "%s:\t%s%s%s", title, k, sep, m[k])
		} else {
			d.line(level, "\t%s%s%s", k, sep, m[k])
		}
	}
}

func (d *describer) meta(m simulator.ObjectMeta, namespaced bool) {
	d.line(0, "Name:\t%s", m.Name)
	if namespaced {
		d.line(0, "Namespace:\t%s", m.Namespace)
	}
}

func (d *describer) labelsAnnotations(m simulator.ObjectMeta) {
	d.multiline(0, "Labels", m.Labels, "=")
	d.multiline(0, "Annotations", m.Annotations, ": ")
}

func controlledBy(m simulator.ObjectMeta) string {
	for _, r := range m.OwnerReferences {
		if r.Controller != nil && *r.Controller {
			return r.Kind + "/" + r.Name
		}
	}
	return ""
}

func (d *describer) events(env Env, kind, ns, name string) {
	var evs []simulator.Event
	for _, e := range allEvents(env) {
		if e.Kind == kind && e.Name == name && (e.Namespace == ns || kind == simulator.KindNode) {
			evs = append(evs, e)
		}
	}
	if len(evs) == 0 {
		d.line(0, "Events:\t<none>")
		return
	}
	d.line(0, "Events:")
	d.line(1, "Type\tReason\tAge\tFrom\tMessage")
	d.line(1, "----\t------\t----\t----\t-------")
	for _, e := range evs {
		d.line(1, "%s\t%s\t%s\t%s\t%s", e.Type, e.Reason, age(d.now, e.Time), eventSource(e.Reason), e.Message)
	}
}

func portsString(ports []simulator.ContainerPort) string {
	if len(ports) == 0 {
		return "<none>"
	}
	var out []string
	for _, p := range ports {
		proto := p.Protocol
		if proto == "" {
			proto = "TCP"
		}
		out = append(out, fmt.Sprintf("%d/%s", p.ContainerPort, proto))
	}
	return strings.Join(out, ", ")
}

func (d *describer) state(level int, title string, s simulator.ContainerState) {
	switch {
	case s.Running != nil:
		d.line(level, "%s:\tRunning", title)
		d.line(level+1, "Started:\t%s", s.Running.StartedAt.Format(describeTime))
	case s.Waiting != nil:
		d.line(level, "%s:\tWaiting", title)
		d.line(level+1, "Reason:\t%s", s.Waiting.Reason)
	case s.Terminated != nil:
		d.line(level, "%s:\tTerminated", title)
		d.line(level+1, "Reason:\t%s", s.Terminated.Reason)
		d.line(level+1, "Exit Code:\t%d", s.Terminated.ExitCode)
		d.line(level+1, "Started:\t%s", s.Terminated.StartedAt.Format(describeTime))
		d.line(level+1, "Finished:\t%s", s.Terminated.FinishedAt.Format(describeTime))
	default:
		d.line(level, "%s:\t<none>", title)
	}
}

// containers prints a pod spec's containers; status is nil inside a Pod Template. As in kubectl,
// names sit at level 1 (one extra space under a template's level-1 heading) and fields at level 2.
func (d *describer) containers(level int, spec simulator.PodSpec, statuses []simulator.ContainerStatus) {
	byName := map[string]simulator.ContainerStatus{}
	for _, cs := range statuses {
		byName[cs.Name] = cs
	}
	d.line(level, "Containers:")
	space := ""
	if level > 0 {
		space = " "
	}
	for _, c := range spec.Containers {
		d.line(1, "%s%s:", space, c.Name)
		in := 2
		d.line(in, "Image:\t%s", c.Image)
		d.line(in, "Port:\t%s", portsString(c.Ports))
		d.line(in, "Host Port:\t%s", hostPorts(c.Ports))
		if len(c.Command) > 0 {
			d.line(in, "Command:")
			for _, a := range c.Command {
				d.line(in+1, "%s", a)
			}
		}
		if len(c.Args) > 0 {
			d.line(in, "Args:")
			for _, a := range c.Args {
				d.line(in+1, "%s", a)
			}
		}
		if statuses != nil {
			cs := byName[c.Name]
			d.state(in, "State", cs.State)
			if cs.LastState.Terminated != nil {
				d.state(in, "Last State", cs.LastState)
			}
			d.line(in, "Ready:\t%s", titleBool(cs.Ready))
			d.line(in, "Restart Count:\t%d", cs.RestartCount)
		}
		d.resources(in, "Limits", c.Resources.Limits)
		d.resources(in, "Requests", c.Resources.Requests)
		d.env(in, c)
		d.line(in, "Mounts:\t<none>")
	}
}

func hostPorts(ports []simulator.ContainerPort) string {
	if len(ports) == 0 {
		return "<none>"
	}
	var out []string
	for _, p := range ports {
		proto := p.Protocol
		if proto == "" {
			proto = "TCP"
		}
		out = append(out, "0/"+proto)
	}
	return strings.Join(out, ", ")
}

func (d *describer) resources(level int, title string, m map[string]string) {
	if len(m) == 0 {
		return
	}
	d.line(level, "%s:", title)
	for _, k := range []string{"cpu", "memory"} {
		if v, ok := m[k]; ok {
			d.line(level+1, "%s:\t%s", k, v)
		}
	}
}

func optionalString(b *bool) string {
	return fmt.Sprintf("%t", b != nil && *b)
}

func (d *describer) env(level int, c simulator.Container) {
	if len(c.EnvFrom) > 0 {
		d.line(level, "Environment Variables from:")
		for _, e := range c.EnvFrom {
			if e.ConfigMapRef != nil {
				d.line(level+1, "%s\tConfigMap\tOptional: %s", e.ConfigMapRef.Name, optionalString(e.ConfigMapRef.Optional))
			}
			if e.SecretRef != nil {
				d.line(level+1, "%s\tSecret\tOptional: %s", e.SecretRef.Name, optionalString(e.SecretRef.Optional))
			}
		}
	}
	if len(c.Env) == 0 {
		d.line(level, "Environment:\t<none>")
		return
	}
	d.line(level, "Environment:")
	for _, e := range c.Env {
		switch {
		case e.ValueFrom != nil && e.ValueFrom.ConfigMapKeyRef != nil:
			r := e.ValueFrom.ConfigMapKeyRef
			d.line(level+1, "%s:\t<set to the key '%s' of config map '%s'>\tOptional: %s", e.Name, r.Key, r.Name, optionalString(r.Optional))
		case e.ValueFrom != nil && e.ValueFrom.SecretKeyRef != nil:
			r := e.ValueFrom.SecretKeyRef
			d.line(level+1, "%s:\t<set to the key '%s' in secret '%s'>\tOptional: %s", e.Name, r.Key, r.Name, optionalString(r.Optional))
		default:
			d.line(level+1, "%s:\t%s", e.Name, e.Value)
		}
	}
}

func titleBool(b bool) string {
	if b {
		return "True"
	}
	return "False"
}

func qosClass(spec simulator.PodSpec) string {
	anyResources, guaranteed := false, true
	for _, c := range spec.Containers {
		if len(c.Resources.Requests) > 0 || len(c.Resources.Limits) > 0 {
			anyResources = true
		}
		for _, r := range []string{"cpu", "memory"} {
			l, ok := c.Resources.Limits[r]
			if !ok {
				guaranteed = false
				continue
			}
			if q, ok := c.Resources.Requests[r]; ok && q != l {
				guaranteed = false
			}
		}
	}
	switch {
	case !anyResources:
		return "BestEffort"
	case guaranteed:
		return "Guaranteed"
	}
	return "Burstable"
}

func describePod(d *describer, env Env, p *simulator.Pod) {
	d.meta(p.Metadata, true)
	d.line(0, "Priority:\t0")
	d.line(0, "Service Account:\tdefault")
	node := "<none>"
	if p.Spec.NodeName != "" {
		node = p.Spec.NodeName
		if p.Status.HostIP != "" {
			node += "/" + p.Status.HostIP
		}
	}
	d.line(0, "Node:\t%s", node)
	if p.Status.StartTime != nil {
		d.line(0, "Start Time:\t%s", p.Status.StartTime.Format(describeTime))
	}
	d.labelsAnnotations(p.Metadata)
	d.line(0, "Status:\t%s", orDefault(p.Status.Phase, "Pending"))
	d.line(0, "IP:\t%s", p.Status.PodIP)
	if p.Status.PodIP != "" {
		d.line(0, "IPs:")
		d.line(1, "IP:\t%s", p.Status.PodIP)
	} else {
		d.line(0, "IPs:\t<none>")
	}
	if c := controlledBy(p.Metadata); c != "" {
		d.line(0, "Controlled By:\t%s", c)
	}
	var statuses []simulator.ContainerStatus
	if len(p.Status.ContainerStatuses) > 0 {
		statuses = p.Status.ContainerStatuses
	} else {
		statuses = []simulator.ContainerStatus{}
	}
	d.containers(0, p.Spec, statuses)
	if len(p.Status.Conditions) > 0 {
		d.line(0, "Conditions:")
		d.line(1, "Type\tStatus")
		for _, c := range p.Status.Conditions {
			d.line(1, "%s \t%s ", c.Type, c.Status)
		}
	}
	d.line(0, "Volumes:\t<none>")
	d.line(0, "QoS Class:\t%s", qosClass(p.Spec))
	d.multiline(0, "Node-Selectors", p.Spec.NodeSelector, "=")
	d.tolerations(p.Spec.Tolerations)
	d.events(env, simulator.KindPod, p.Metadata.Namespace, p.Metadata.Name)
}

func (d *describer) tolerations(ts []simulator.Toleration) {
	if len(ts) == 0 {
		d.line(0, "Tolerations:\t<none>")
		return
	}
	for i, t := range ts {
		s := t.Key
		if t.Operator == "Exists" && t.Key == "" {
			s = "op=Exists"
		} else if t.Value != "" {
			s += "=" + t.Value
		}
		if t.Effect != "" {
			s += ":" + t.Effect
		}
		if t.Operator == "Exists" && t.Key != "" {
			s += " op=Exists"
		}
		if i == 0 {
			d.line(0, "Tolerations:\t%s", s)
		} else {
			d.line(0, "\t%s", s)
		}
	}
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func (d *describer) podTemplate(t simulator.PodTemplateSpec) {
	d.line(0, "Pod Template:")
	d.multiline(1, "Labels", t.Metadata.Labels, "=")
	if len(t.Metadata.Annotations) > 0 {
		d.multiline(1, "Annotations", t.Metadata.Annotations, ": ")
	}
	d.containers(1, t.Spec, nil)
	d.line(1, "Volumes:\t<none>")
	d.multiline(1, "Node-Selectors", t.Spec.NodeSelector, "=")
	if len(t.Spec.Tolerations) == 0 {
		d.line(1, "Tolerations:\t<none>")
	}
}

func intOrPercent(v *simulator.IntOrString, def string) string {
	if v == nil {
		return def
	}
	if v.IsStr {
		return v.StrVal
	}
	return fmt.Sprintf("%d", v.IntVal)
}

func describeDeployment(d *describer, env Env, dep *simulator.Deployment) {
	d.meta(dep.Metadata, true)
	d.line(0, "CreationTimestamp:\t%s", dep.Metadata.CreationTimestamp.Format(describeTime))
	d.labelsAnnotations(dep.Metadata)
	d.line(0, "Selector:\t%s", selectorString(dep.Spec.Selector))
	s := dep.Status
	d.line(0, "Replicas:\t%d desired | %d updated | %d total | %d available | %d unavailable",
		dep.DesiredReplicas(), s.UpdatedReplicas, s.Replicas, s.AvailableReplicas, s.UnavailableReplicas)
	strategy := orDefault(dep.Spec.Strategy.Type, "RollingUpdate")
	d.line(0, "StrategyType:\t%s", strategy)
	d.line(0, "MinReadySeconds:\t0")
	if strategy == "RollingUpdate" {
		var ru simulator.RollingUpdate
		if dep.Spec.Strategy.RollingUpdate != nil {
			ru = *dep.Spec.Strategy.RollingUpdate
		}
		d.line(0, "RollingUpdateStrategy:\t%s max unavailable, %s max surge", intOrPercent(ru.MaxUnavailable, "25%"), intOrPercent(ru.MaxSurge, "25%"))
	}
	d.podTemplate(dep.Spec.Template)
	if len(s.Conditions) > 0 {
		d.line(0, "Conditions:")
		d.line(1, "Type\tStatus\tReason")
		d.line(1, "----\t------\t------")
		for _, c := range s.Conditions {
			d.line(1, "%s \t%s\t%s", c.Type, c.Status, c.Reason)
		}
	}
	var oldRS []string
	newRS := "<none>"
	hash := simulator.TemplateHash(dep.Spec.Template)
	for _, o := range env.State.List(simulator.KindReplicaSet, dep.Metadata.Namespace) {
		rs := o.(*simulator.ReplicaSet)
		if controlledBy(rs.Metadata) != "Deployment/"+dep.Metadata.Name {
			continue
		}
		desc := fmt.Sprintf("%s (%d/%d replicas created)", rs.Metadata.Name, rs.Status.Replicas, rs.DesiredReplicas())
		if rs.Metadata.Labels[simulator.PodTemplateHashLabel] == hash {
			newRS = desc
		} else if rs.DesiredReplicas() > 0 || rs.Status.Replicas > 0 {
			oldRS = append(oldRS, desc)
		}
	}
	if len(oldRS) == 0 {
		d.line(0, "OldReplicaSets:\t<none>")
	} else {
		d.line(0, "OldReplicaSets:\t%s", strings.Join(oldRS, ", "))
	}
	d.line(0, "NewReplicaSet:\t%s", newRS)
	d.events(env, simulator.KindDeployment, dep.Metadata.Namespace, dep.Metadata.Name)
}

func describeReplicaSet(d *describer, env Env, rs *simulator.ReplicaSet) {
	d.meta(rs.Metadata, true)
	d.line(0, "Selector:\t%s", selectorString(rs.Spec.Selector))
	d.labelsAnnotations(rs.Metadata)
	if c := controlledBy(rs.Metadata); c != "" {
		d.line(0, "Controlled By:\t%s", c)
	}
	d.line(0, "Replicas:\t%d current / %d desired", rs.Status.Replicas, rs.DesiredReplicas())
	running, waiting := 0, 0
	for _, o := range env.State.List(simulator.KindPod, rs.Metadata.Namespace) {
		p := o.(*simulator.Pod)
		if controlledBy(p.Metadata) != "ReplicaSet/"+rs.Metadata.Name {
			continue
		}
		if p.Status.Phase == "Running" {
			running++
		} else {
			waiting++
		}
	}
	d.line(0, "Pods Status:\t%d Running / %d Waiting / 0 Succeeded / 0 Failed", running, waiting)
	d.podTemplate(rs.Spec.Template)
	d.events(env, simulator.KindReplicaSet, rs.Metadata.Namespace, rs.Metadata.Name)
}

func describeService(d *describer, env Env, svc *simulator.Service) {
	d.meta(svc.Metadata, true)
	d.labelsAnnotations(svc.Metadata)
	d.line(0, "Selector:\t%s", labelString(svc.Spec.Selector))
	d.line(0, "Type:\t%s", orDefault(svc.Spec.Type, "ClusterIP"))
	d.line(0, "IP Family Policy:\tSingleStack")
	d.line(0, "IP Families:\tIPv4")
	d.line(0, "IP:\t%s", svc.Spec.ClusterIP)
	d.line(0, "IPs:\t%s", strings.Join(svc.Spec.ClusterIPs, ","))
	if svc.Spec.Type == "LoadBalancer" {
		d.line(0, "LoadBalancer Ingress:\t<pending>")
	}
	endpoints := endpointsOf(env.State, svc)
	for _, p := range svc.Spec.Ports {
		proto := orDefault(p.Protocol, "TCP")
		d.line(0, "Port:\t%s\t%d/%s", orDefault(p.Name, "<unset>"), p.Port, proto)
		target := fmt.Sprintf("%d/%s", p.TargetPort.IntVal, proto)
		if p.TargetPort.IsStr {
			target = p.TargetPort.StrVal + "/" + proto
		}
		d.line(0, "TargetPort:\t%s", target)
		if p.NodePort != 0 {
			d.line(0, "NodePort:\t%s\t%d/%s", orDefault(p.Name, "<unset>"), p.NodePort, proto)
		}
		d.line(0, "Endpoints:\t%s", formatAddresses(endpoints))
	}
	d.line(0, "Session Affinity:\tNone")
	if svc.Spec.Type == "NodePort" || svc.Spec.Type == "LoadBalancer" {
		d.line(0, "External Traffic Policy:\tCluster")
	}
	d.line(0, "Internal Traffic Policy:\tCluster")
	d.events(env, simulator.KindService, svc.Metadata.Namespace, svc.Metadata.Name)
}

func describeNode(d *describer, env Env, n *simulator.Node) {
	d.line(0, "Name:\t%s", n.Metadata.Name)
	d.line(0, "Roles:\t%s", nodeRoles(n))
	d.labelsAnnotations(n.Metadata)
	d.line(0, "CreationTimestamp:\t%s", n.Metadata.CreationTimestamp.Format(describeTime))
	if len(n.Spec.Taints) == 0 {
		d.line(0, "Taints:\t<none>")
	}
	for i, t := range n.Spec.Taints {
		s := t.Key
		if t.Value != "" {
			s += "=" + t.Value
		}
		s += ":" + t.Effect
		if i == 0 {
			d.line(0, "Taints:\t%s", s)
		} else {
			d.line(0, "\t%s", s)
		}
	}
	d.line(0, "Unschedulable:\t%t", n.Spec.Unschedulable)
	d.line(0, "Conditions:")
	d.line(1, "Type\tStatus")
	d.line(1, "----\t------")
	for _, c := range n.Status.Conditions {
		d.line(1, "%s\t%s", c.Type, c.Status)
	}
	d.line(0, "Addresses:")
	for _, a := range n.Status.Addresses {
		d.line(1, "%s:\t%s", a.Type, a.Address)
	}
	for _, sec := range []struct {
		title string
		m     map[string]string
	}{{"Capacity", n.Status.Capacity}, {"Allocatable", n.Status.Allocatable}} {
		d.line(0, "%s:", sec.title)
		for _, k := range []string{"cpu", "memory", "pods"} {
			if v, ok := sec.m[k]; ok {
				d.line(1, "%s:\t%s", k, v)
			}
		}
	}
	if info := n.Status.NodeInfo; info != nil {
		d.line(0, "System Info:")
		d.line(1, "Kubelet Version:\t%s", info.KubeletVersion)
		d.line(1, "Container Runtime Version:\t%s", info.ContainerRuntimeVersion)
		d.line(1, "OS Image:\t%s", info.OSImage)
		d.line(1, "Operating System:\t%s", info.OperatingSystem)
		d.line(1, "Architecture:\t%s", info.Architecture)
	}
	var pods []*simulator.Pod
	for _, o := range env.State.List(simulator.KindPod, "*") {
		if p := o.(*simulator.Pod); p.Spec.NodeName == n.Metadata.Name {
			pods = append(pods, p)
		}
	}
	d.line(0, "Non-terminated Pods:\t(%d in total)", len(pods))
	alloc := n.Status.Allocatable
	if len(alloc) == 0 {
		alloc = n.Status.Capacity
	}
	allocCPU, _ := simulator.ParseCPU(alloc["cpu"])
	allocMem, _ := simulator.ParseMemory(alloc["memory"])
	pct := func(v, total int64) int64 {
		if total == 0 {
			return 0
		}
		return v * 100 / total
	}
	var cpuReq, memReq int64
	if len(pods) > 0 {
		d.line(1, "Namespace\tName\tCPU Requests\tMemory Requests\tAge")
		d.line(1, "---------\t----\t------------\t---------------\t---")
		for _, p := range pods {
			var c, m int64
			for _, ctr := range p.Spec.Containers {
				v, _ := simulator.ParseCPU(ctr.Resources.Requests["cpu"])
				c += v
				mv, _ := simulator.ParseMemory(ctr.Resources.Requests["memory"])
				m += mv
			}
			cpuReq += c
			memReq += m
			d.line(1, "%s\t%s\t%s (%d%%)\t%s (%d%%)\t%s", p.Metadata.Namespace, p.Metadata.Name,
				simulator.FormatCPU(c), pct(c, allocCPU), simulator.FormatMemory(m), pct(m, allocMem), age(d.now, p.Metadata.CreationTimestamp))
		}
	}
	d.line(0, "Allocated resources:")
	d.line(1, "(Total limits may be over 100 percent, i.e., overcommitted.)")
	d.line(1, "Resource\tRequests")
	d.line(1, "--------\t--------")
	d.line(1, "cpu\t%s (%d%%)", simulator.FormatCPU(cpuReq), pct(cpuReq, allocCPU))
	d.line(1, "memory\t%s (%d%%)", simulator.FormatMemory(memReq), pct(memReq, allocMem))
	d.events(env, simulator.KindNode, "", n.Metadata.Name)
}

func describeNamespace(d *describer, _ Env, ns *simulator.Namespace) {
	d.line(0, "Name:\t%s", ns.Metadata.Name)
	d.labelsAnnotations(ns.Metadata)
	d.line(0, "Status:\t%s", orDefault(ns.Status.Phase, "Active"))
	d.line(0, "")
	d.line(0, "No resource quota.")
	d.line(0, "")
	d.line(0, "No LimitRange resource.")
}

func describeConfigMap(d *describer, env Env, cm *simulator.ConfigMap) {
	d.meta(cm.Metadata, true)
	d.labelsAnnotations(cm.Metadata)
	d.line(0, "")
	d.line(0, "Data")
	d.line(0, "====")
	keys := make([]string, 0, len(cm.Data))
	for k := range cm.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		d.line(0, "%s:", k)
		d.line(0, "----")
		d.line(0, "%s", cm.Data[k])
		d.line(0, "")
	}
	d.line(0, "")
	d.line(0, "BinaryData")
	d.line(0, "====")
	d.line(0, "")
	d.events(env, simulator.KindConfigMap, cm.Metadata.Namespace, cm.Metadata.Name)
}

func describeSecret(d *describer, _ Env, s *simulator.Secret) {
	d.meta(s.Metadata, true)
	d.labelsAnnotations(s.Metadata)
	d.line(0, "")
	d.line(0, "Type:\t%s", s.Type)
	d.line(0, "")
	d.line(0, "Data")
	d.line(0, "====")
	keys := make([]string, 0, len(s.Data))
	for k := range s.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		// Like kubectl, describe never prints Secret values.
		d.line(0, "%s:\t%d bytes", k, len(s.Data[k]))
	}
}

func describeEndpointSlice(d *describer, env Env, s *simulator.EndpointSlice) {
	d.meta(s.Metadata, true)
	d.labelsAnnotations(s.Metadata)
	d.line(0, "AddressType:\t%s", s.AddressType)
	d.line(0, "Ports:")
	d.line(1, "Name\tPort\tProtocol")
	d.line(1, "----\t----\t--------")
	for _, p := range s.Ports {
		d.line(1, "%s\t%d\t%s", orDefault(p.Name, "<unset>"), p.Port, p.Protocol)
	}
	d.line(0, "Endpoints:")
	for _, e := range s.Endpoints {
		d.line(1, "- Addresses:\t%s", strings.Join(e.Addresses, ","))
		d.line(2, "Conditions:")
		d.line(3, "Ready:\t%t", e.Conditions.Ready)
		if e.TargetRef != nil {
			d.line(2, "TargetRef:\t%s/%s", e.TargetRef.Kind, e.TargetRef.Name)
		}
		d.line(2, "NodeName:\t%s", e.NodeName)
	}
	d.events(env, simulator.KindEndpointSlice, s.Metadata.Namespace, s.Metadata.Name)
}

func describeEndpoints(d *describer, env Env, it item) {
	d.line(0, "Name:\t%s", it.name)
	d.line(0, "Namespace:\t%s", it.ns)
	d.multiline(0, "Labels", it.labels, "=")
	d.line(0, "Annotations:\t<none>")
	d.line(0, "Subsets:")
	if len(it.endpoints) > 0 {
		var ips []string
		for _, e := range it.endpoints {
			ip, _, _ := strings.Cut(e, ":")
			ips = append(ips, ip)
		}
		d.line(1, "Addresses:\t%s", strings.Join(ips, ","))
	}
	d.events(env, kindEndpoints, it.ns, it.name)
}

func describeItem(env Env, it item) string {
	d := &describer{now: env.State.Now}
	switch o := it.obj.(type) {
	case *simulator.Pod:
		describePod(d, env, o)
	case *simulator.Deployment:
		describeDeployment(d, env, o)
	case *simulator.ReplicaSet:
		describeReplicaSet(d, env, o)
	case *simulator.Service:
		describeService(d, env, o)
	case *simulator.Node:
		describeNode(d, env, o)
	case *simulator.Namespace:
		describeNamespace(d, env, o)
	case *simulator.ConfigMap:
		describeConfigMap(d, env, o)
	case *simulator.Secret:
		describeSecret(d, env, o)
	case *simulator.EndpointSlice:
		describeEndpointSlice(d, env, o)
	default:
		if it.endpoints != nil || it.t.Kind == kindEndpoints {
			describeEndpoints(d, env, it)
		}
	}
	return d.String()
}

var describeFlags = []flagDef{{Long: "selector", Short: "l", Value: true}, {Long: "show-events"}}

func runDescribe(args []string, env Env, p *Parsed) (Result, *Failure) {
	pos, fl, f := parseFlags(args, "describe", describeFlags)
	if f != nil {
		return Result{}, f
	}
	if fl.boolean("help") {
		return Result{Stdout: commandHelp["describe"]}, nil
	}
	recordFlags(p, fl)
	sel, err := simulator.ParseSelector(fl.str("selector"))
	if err != nil {
		return Result{}, reject(CodeValidation, "error: %s", err.Error())
	}
	targets, f := resolveTargets(pos, "describe")
	if f != nil {
		return Result{}, f
	}
	ns, all := namespaceFor(fl, env), fl.boolean("all-namespaces")
	p.Namespace, p.AllNamespaces, p.Selector = ns, all, fl.str("selector")
	var out []string
	var stderr strings.Builder
	for _, tg := range targets {
		p.Resources = append(p.Resources, tg.t.Kind)
		p.Names = append(p.Names, tg.names...)
		if tg.t.Kind == kindEvent {
			return Result{}, reject(CodeUnsupported, "error: describe events isn't supported; use 'kubectl events' or 'kubectl get events'")
		}
		items := listItems(env, tg.t, ns, all)
		if len(tg.names) == 0 {
			for _, it := range items {
				if sel.Matches(it.labels) {
					out = append(out, describeItem(env, it))
				}
			}
			continue
		}
		for _, name := range tg.names {
			// Like kubectl, an exact name wins; otherwise every name with that prefix matches.
			var matched []item
			for _, it := range items {
				if it.name == name {
					matched = []item{it}
					break
				}
				if strings.HasPrefix(it.name, name) {
					matched = append(matched, it)
				}
			}
			if len(matched) == 0 {
				stderr.WriteString(notFound(tg.t, name))
				if tg.t.Namespaced && !all {
					stderr.WriteString(namespaceHint(env, ns))
				}
			}
			for _, it := range matched {
				out = append(out, describeItem(env, it))
			}
		}
	}
	res := Result{Stdout: strings.Join(out, "\n\n"), Stderr: stderr.String()}
	if len(out) == 0 && stderr.Len() == 0 {
		if all {
			res.Stderr = "No resources found\n"
		} else {
			res.Stderr = fmt.Sprintf("No resources found in %s namespace.\n", ns) + namespaceHint(env, ns)
		}
	}
	if strings.Contains(res.Stderr, "NotFound") {
		res.ExitCode, res.ErrorCode = 1, CodeNotFound
	}
	return res, nil
}
