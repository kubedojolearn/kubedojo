package simulator

import (
	"fmt"
	"net/netip"
	"time"
)

// crashBackoff returns how many restarts fit in elapsed under kubelet's backoff (10s doubling
// to a 5m cap), the current backoff delay, and when (after start) the last restart happened.
func crashBackoff(elapsed time.Duration) (restarts int32, delay, last time.Duration) {
	delay = 10 * time.Second
	for last+delay <= elapsed && restarts < 1_000_000 {
		last += delay
		restarts++
		delay = min(delay*2, 5*time.Minute)
	}
	return restarts, delay, last
}

type configProblem struct{ message string }

// resolveEnv returns the names of environment variables the container would get, or the first
// problem resolving its ConfigMap/Secret references (CreateContainerConfigError).
func resolveEnv(s *State, ns string, c Container) (map[string]bool, *configProblem) {
	names := map[string]bool{}
	optional := func(b *bool) bool { return b != nil && *b }
	for _, src := range c.EnvFrom {
		switch {
		case src.ConfigMapRef != nil:
			cm, ok := s.Get(KindConfigMap, ns, src.ConfigMapRef.Name).(*ConfigMap)
			if !ok {
				if optional(src.ConfigMapRef.Optional) {
					continue
				}
				return nil, &configProblem{fmt.Sprintf("configmap %q not found", src.ConfigMapRef.Name)}
			}
			for k := range cm.Data {
				names[k] = true
			}
		case src.SecretRef != nil:
			sec, ok := s.Get(KindSecret, ns, src.SecretRef.Name).(*Secret)
			if !ok {
				if optional(src.SecretRef.Optional) {
					continue
				}
				return nil, &configProblem{fmt.Sprintf("secret %q not found", src.SecretRef.Name)}
			}
			for k := range sec.Data {
				names[k] = true
			}
		}
	}
	for _, e := range c.Env {
		if e.ValueFrom == nil {
			names[e.Name] = true
			continue
		}
		if ref := e.ValueFrom.ConfigMapKeyRef; ref != nil {
			cm, ok := s.Get(KindConfigMap, ns, ref.Name).(*ConfigMap)
			switch {
			case !ok && optional(ref.Optional):
				continue
			case !ok:
				return nil, &configProblem{fmt.Sprintf("configmap %q not found", ref.Name)}
			}
			if _, has := cm.Data[ref.Key]; !has {
				if optional(ref.Optional) {
					continue
				}
				return nil, &configProblem{fmt.Sprintf("couldn't find key %s in ConfigMap %s/%s", ref.Key, ns, ref.Name)}
			}
		}
		if ref := e.ValueFrom.SecretKeyRef; ref != nil {
			sec, ok := s.Get(KindSecret, ns, ref.Name).(*Secret)
			switch {
			case !ok && optional(ref.Optional):
				continue
			case !ok:
				return nil, &configProblem{fmt.Sprintf("secret %q not found", ref.Name)}
			}
			if _, has := sec.Data[ref.Key]; !has {
				if optional(ref.Optional) {
					continue
				}
				return nil, &configProblem{fmt.Sprintf("couldn't find key %s in Secret %s/%s", ref.Key, ns, ref.Name)}
			}
		}
		names[e.Name] = true
	}
	return names, nil
}

// ContainerCrashes reports whether a container with this image profile and environment crashes.
func ContainerCrashes(p ImageProfile, env map[string]bool) bool {
	if p.Behavior != "crash" {
		return false
	}
	if len(p.RequiresEnv) == 0 {
		return true
	}
	for _, name := range p.RequiresEnv {
		if !env[name] {
			return true
		}
	}
	return false
}

func runKubelets(s *State) error {
	for _, o := range s.List(KindPod, "*") {
		p := o.(*Pod)
		if p.Spec.NodeName == "" {
			continue
		}
		node, ok := s.Get(KindNode, "", p.Spec.NodeName).(*Node)
		if !ok {
			continue // bound to a node that doesn't exist: the Pod stays as it is
		}
		if err := syncPod(s, p, node); err != nil {
			return err
		}
	}
	return nil
}

func syncPod(s *State, p *Pod, node *Node) error {
	before := p.DeepCopy().(*Pod)
	ns := p.Metadata.Namespace
	if p.Status.StartTime == nil {
		t := s.Now
		p.Status.StartTime = &t
	}
	p.Status.HostIP = nodeInternalIP(node)
	if p.Status.PodIP == "" {
		ip, err := allocatePodIP(s, node.Metadata.Name)
		if err != nil {
			return err
		}
		p.Status.PodIP = ip
	}
	elapsed := s.Now.Sub(*p.Status.StartTime)

	previous := map[string]ContainerStatus{}
	for _, cs := range p.Status.ContainerStatuses {
		previous[cs.Name] = cs
	}
	var statuses []ContainerStatus
	allReady, anyWaitingStart := true, false
	for _, c := range p.Spec.Containers {
		prev := previous[c.Name]
		cs := ContainerStatus{Name: c.Name, Image: c.Image}
		profile, found := s.Images.Lookup(c.Image)
		env, problem := resolveEnv(s, ns, c)
		switch {
		case !found:
			cs.State.Waiting = &ContainerStateWaiting{Reason: "ImagePullBackOff", Message: fmt.Sprintf("Back-off pulling image %q", c.Image)}
			anyWaitingStart = true
			if prev.State.Waiting == nil || prev.State.Waiting.Reason != "ImagePullBackOff" {
				s.Record("Normal", "Pulling", p, fmt.Sprintf("Pulling image %q", c.Image))
				s.Record("Warning", "Failed", p, fmt.Sprintf("Failed to pull image %q: rpc error: code = NotFound desc = failed to pull and unpack image %q: not found (simulated registry)", c.Image, c.Image))
				s.Record("Warning", "Failed", p, "Error: ErrImagePull")
				s.Record("Normal", "BackOff", p, fmt.Sprintf("Back-off pulling image %q", c.Image))
				s.Record("Warning", "Failed", p, "Error: ImagePullBackOff")
			}
		case problem != nil:
			cs.State.Waiting = &ContainerStateWaiting{Reason: "CreateContainerConfigError", Message: problem.message}
			anyWaitingStart = true
			if prev.State.Waiting == nil || prev.State.Waiting.Message != problem.message {
				s.Record("Normal", "Pulled", p, fmt.Sprintf("Container image %q already present on machine (simulated)", c.Image))
				s.Record("Warning", "Failed", p, "Error: "+problem.message)
			}
		case ContainerCrashes(profile, env):
			// Each attempt crashes as soon as it starts; the last one began at the last restart.
			restarts, delay, last := crashBackoff(elapsed)
			started := p.Status.StartTime.Add(last)
			cs.RestartCount = restarts
			cs.Started = false
			cs.State.Waiting = &ContainerStateWaiting{
				Reason:  "CrashLoopBackOff",
				Message: fmt.Sprintf("back-off %s restarting failed container=%s pod=%s_%s(%s)", delay, c.Name, p.Metadata.Name, ns, p.Metadata.UID),
			}
			cs.LastState.Terminated = &ContainerStateTerminated{ExitCode: profile.ExitCode, Reason: "Error", StartedAt: started, FinishedAt: started}
			if prev.State.Waiting == nil || prev.State.Waiting.Reason != "CrashLoopBackOff" {
				s.Record("Normal", "Pulled", p, fmt.Sprintf("Container image %q already present on machine (simulated)", c.Image))
				s.Record("Normal", "Created", p, "Created container: "+c.Name)
				s.Record("Normal", "Started", p, "Started container "+c.Name)
				s.Record("Warning", "BackOff", p, fmt.Sprintf("Back-off restarting failed container %s in pod %s_%s(%s)", c.Name, p.Metadata.Name, ns, p.Metadata.UID))
			}
		default:
			startedAt := s.Now
			if prev.State.Running != nil {
				startedAt = prev.State.Running.StartedAt
			}
			cs.Ready, cs.Started = true, true
			cs.RestartCount = prev.RestartCount
			cs.State.Running = &ContainerStateRunning{StartedAt: startedAt}
			if prev.State.Running == nil {
				s.Record("Normal", "Pulled", p, fmt.Sprintf("Container image %q already present on machine (simulated)", c.Image))
				s.Record("Normal", "Created", p, "Created container: "+c.Name)
				s.Record("Normal", "Started", p, "Started container "+c.Name)
			}
		}
		if !cs.Ready {
			allReady = false
		}
		statuses = append(statuses, cs)
	}
	p.Status.ContainerStatuses = statuses

	if anyWaitingStart {
		p.Status.Phase = "Pending"
	} else {
		p.Status.Phase = "Running"
	}
	ready := "False"
	reason := "ContainersNotReady"
	if allReady {
		ready, reason = "True", ""
	}
	msg := ""
	if !allReady {
		msg = "containers with unready status: ["
		first := true
		for _, cs := range statuses {
			if !cs.Ready {
				if !first {
					msg += " "
				}
				msg += cs.Name
				first = false
			}
		}
		msg += "]"
	}
	setPodCondition(p, PodCondition{Type: "PodReadyToStartContainers", Status: "True", LastTransitionTime: s.Now})
	setPodCondition(p, PodCondition{Type: "Initialized", Status: "True", LastTransitionTime: s.Now})
	setPodCondition(p, PodCondition{Type: "Ready", Status: ready, Reason: reason, Message: msg, LastTransitionTime: s.Now})
	setPodCondition(p, PodCondition{Type: "ContainersReady", Status: ready, Reason: reason, Message: msg, LastTransitionTime: s.Now})
	setPodCondition(p, PodCondition{Type: "PodScheduled", Status: "True", LastTransitionTime: s.Now})
	orderPodConditions(p)

	if jsonEqual(before, p) {
		return nil
	}
	return s.Put(p)
}

var podConditionOrder = map[string]int{"PodReadyToStartContainers": 0, "Initialized": 1, "Ready": 2, "ContainersReady": 3, "PodScheduled": 4}

func orderPodConditions(p *Pod) {
	cs := p.Status.Conditions
	for i := 1; i < len(cs); i++ {
		for j := i; j > 0 && podConditionOrder[cs[j].Type] < podConditionOrder[cs[j-1].Type]; j-- {
			cs[j], cs[j-1] = cs[j-1], cs[j]
		}
	}
}

// allocatePodIP returns the lowest free address in the node's Pod CIDR, 10.244.<index>.0/24.
func allocatePodIP(s *State, nodeName string) (string, error) {
	idx := nodeIndex(s, nodeName)
	used := map[string]bool{}
	for _, o := range s.List(KindPod, "*") {
		if p := o.(*Pod); p.Status.PodIP != "" {
			used[p.Status.PodIP] = true
		}
	}
	addr := netip.MustParseAddr(fmt.Sprintf("10.244.%d.2", idx))
	for i := 2; i < 255; i++ {
		if !used[addr.String()] {
			return addr.String(), nil
		}
		addr = addr.Next()
	}
	return "", fmt.Errorf("%w: no free Pod IPs on node %s", ErrLimit, nodeName)
}
