package simulator

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"sort"
	"strconv"
)

const (
	// RevisionAnnotation records a ReplicaSet's (and its Deployment's current) revision.
	RevisionAnnotation = "deployment.kubernetes.io/revision"
	// PodTemplateHashLabel ties ReplicaSets and Pods to a template version.
	PodTemplateHashLabel = "pod-template-hash"
)

// TemplateHash computes a stable, kubectl-looking hash of a pod template (10 characters from
// the generated-name alphabet), so identical templates map to the same ReplicaSet.
func TemplateHash(t PodTemplateSpec) string {
	raw, _ := json.Marshal(t)
	h := fnv.New64a()
	_, _ = h.Write(raw)
	v := h.Sum64()
	b := make([]byte, 10)
	for i := range b {
		b[i] = nameAlphabet[v%uint64(len(nameAlphabet))]
		v /= uint64(len(nameAlphabet))
	}
	return string(b)
}

func revisionOf(o Object) int64 {
	n, _ := strconv.ParseInt(o.Meta().Annotations[RevisionAnnotation], 10, 64)
	return n
}

func ownedBy(s *State, kind, namespace, ownerUID string) []Object {
	var out []Object
	for _, o := range s.List(kind, namespace) {
		if ControllerOf(o) == ownerUID {
			out = append(out, o)
		}
	}
	return out
}

func resolveIntOrPercent(v *IntOrString, total int32, def string, roundUp bool) int32 {
	val := IntOrString{StrVal: def, IsStr: true}
	if v != nil {
		val = *v
	}
	if !val.IsStr {
		return val.IntVal
	}
	pct, err := strconv.Atoi(trimPercent(val.StrVal))
	if err != nil {
		return 0
	}
	f := float64(total) * float64(pct) / 100
	if roundUp {
		return int32(math.Ceil(f))
	}
	return int32(math.Floor(f))
}

func trimPercent(s string) string {
	if len(s) > 0 && s[len(s)-1] == '%' {
		return s[:len(s)-1]
	}
	return s
}

func copyLabels(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func reconcileDeployments(s *State) error {
	for _, o := range s.List(KindDeployment, "*") {
		if err := reconcileDeployment(s, o.(*Deployment)); err != nil {
			return err
		}
	}
	return nil
}

func reconcileDeployment(s *State, d *Deployment) error {
	ns := d.Metadata.Namespace
	desired := d.DesiredReplicas()
	hash := TemplateHash(d.Spec.Template)
	rsName := d.Metadata.Name + "-" + hash

	var owned []*ReplicaSet
	for _, o := range ownedBy(s, KindReplicaSet, ns, d.Metadata.UID) {
		owned = append(owned, o.(*ReplicaSet))
	}
	maxRev := int64(0)
	var current *ReplicaSet
	var old []*ReplicaSet
	for _, rs := range owned {
		maxRev = max(maxRev, revisionOf(rs))
		if rs.Metadata.Labels[PodTemplateHashLabel] == hash {
			current = rs
		} else {
			old = append(old, rs)
		}
	}
	// Oldest revisions first, for scale-down order.
	sort.Slice(old, func(i, j int) bool { return revisionOf(old[i]) < revisionOf(old[j]) })

	recreate := d.Spec.Strategy.Type == "Recreate"
	maxSurge := resolveIntOrPercent(rollingParam(d, true), desired, "25%", true)
	maxUnavailable := resolveIntOrPercent(rollingParam(d, false), desired, "25%", false)
	if maxSurge == 0 && maxUnavailable == 0 {
		maxUnavailable = 1
	}

	if current == nil {
		// Recreate: all old Pods must be gone before the new ReplicaSet appears.
		if recreate {
			for _, rs := range old {
				if rs.DesiredReplicas() > 0 || rs.Status.Replicas > 0 {
					return scaleReplicaSet(s, d, rs, 0)
				}
			}
		}
		if s.Has(KindReplicaSet, ns, rsName) {
			return fmt.Errorf("replica set %s exists but is not owned by deployment %s", rsName, d.Metadata.Name)
		}
		tpl := d.Spec.Template
		tpl.Metadata.Labels = copyLabels(tpl.Metadata.Labels)
		tpl.Metadata.Labels[PodTemplateHashLabel] = hash
		sel := d.Spec.Selector
		sel.MatchLabels = copyLabels(sel.MatchLabels)
		sel.MatchLabels[PodTemplateHashLabel] = hash
		initial := int32(0)
		if len(old) == 0 || recreate {
			initial = desired
		}
		zero := int32(0)
		rs := &ReplicaSet{
			Metadata: ObjectMeta{
				Name: rsName, Namespace: ns, Labels: copyLabels(tpl.Metadata.Labels),
				Annotations:     map[string]string{RevisionAnnotation: strconv.FormatInt(maxRev+1, 10)},
				OwnerReferences: []OwnerReference{controllerRef(d)},
			},
			Spec: ReplicaSetSpec{Replicas: &zero, Selector: sel, Template: tpl},
		}
		if err := s.Put(rs); err != nil {
			return err
		}
		current = s.Get(KindReplicaSet, ns, rsName).(*ReplicaSet)
		if initial > 0 {
			if err := scaleReplicaSet(s, d, current, initial); err != nil {
				return err
			}
		}
		return updateDeploymentRevision(s, d, maxRev+1)
	}

	// A rollback to an older template makes that ReplicaSet the newest revision again.
	if rev := revisionOf(current); rev < maxRev {
		current.Metadata.Annotations[RevisionAnnotation] = strconv.FormatInt(maxRev+1, 10)
		if err := s.Put(current); err != nil {
			return err
		}
		return updateDeploymentRevision(s, d, maxRev+1)
	}
	if err := updateDeploymentRevision(s, d, revisionOf(current)); err != nil {
		return err
	}

	switch {
	case len(old) == 0 || recreate:
		for _, rs := range old {
			if rs.DesiredReplicas() > 0 {
				if err := scaleReplicaSet(s, d, rs, 0); err != nil {
					return err
				}
			}
		}
		if current.DesiredReplicas() != desired {
			if err := scaleReplicaSet(s, d, current, desired); err != nil {
				return err
			}
		}
	default:
		if err := rollingStep(s, d, current, old, desired, maxSurge, maxUnavailable); err != nil {
			return err
		}
	}

	if err := cleanupHistory(s, d, old); err != nil {
		return err
	}
	return updateDeploymentStatus(s, d.Metadata.Namespace, d.Metadata.Name, hash, desired, maxUnavailable)
}

func rollingParam(d *Deployment, surge bool) *IntOrString {
	if d.Spec.Strategy.RollingUpdate == nil {
		return nil
	}
	if surge {
		return d.Spec.Strategy.RollingUpdate.MaxSurge
	}
	return d.Spec.Strategy.RollingUpdate.MaxUnavailable
}

// rollingStep performs one step of a rolling update: scale the new ReplicaSet up within
// maxSurge, or scale old ones down while keeping desired-maxUnavailable Pods available.
func rollingStep(s *State, d *Deployment, current *ReplicaSet, old []*ReplicaSet, desired, maxSurge, maxUnavailable int32) error {
	total := current.DesiredReplicas()
	available := current.Status.AvailableReplicas
	for _, rs := range old {
		total += rs.DesiredReplicas()
		available += rs.Status.AvailableReplicas
	}
	if current.DesiredReplicas() < desired && total < desired+maxSurge {
		next := min(desired, current.DesiredReplicas()+(desired+maxSurge-total))
		return scaleReplicaSet(s, d, current, next)
	}
	if current.DesiredReplicas() > desired {
		return scaleReplicaSet(s, d, current, desired)
	}
	canRemove := available - (desired - maxUnavailable)
	// Unavailable old Pods can always go: they don't count toward availability.
	for _, rs := range old {
		if canRemove <= 0 && rs.Status.AvailableReplicas == rs.DesiredReplicas() {
			continue
		}
		if rs.DesiredReplicas() == 0 {
			continue
		}
		unavailableHere := rs.DesiredReplicas() - rs.Status.AvailableReplicas
		remove := min(rs.DesiredReplicas(), max(canRemove, 0)+unavailableHere)
		if remove <= 0 {
			continue
		}
		return scaleReplicaSet(s, d, rs, rs.DesiredReplicas()-remove)
	}
	return nil
}

func scaleReplicaSet(s *State, d *Deployment, rs *ReplicaSet, to int32) error {
	from := rs.DesiredReplicas()
	if from == to {
		return nil
	}
	rs.Spec.Replicas = &to
	if err := s.Put(rs); err != nil {
		return err
	}
	verb := "up"
	if to < from {
		verb = "down"
	}
	s.Record("Normal", "ScalingReplicaSet", d, fmt.Sprintf("Scaled %s replica set %s from %d to %d", verb, rs.Metadata.Name, from, to))
	return nil
}

func updateDeploymentRevision(s *State, d *Deployment, rev int64) error {
	fresh := s.Get(KindDeployment, d.Metadata.Namespace, d.Metadata.Name).(*Deployment)
	want := strconv.FormatInt(rev, 10)
	if fresh.Metadata.Annotations[RevisionAnnotation] == want {
		return nil
	}
	if fresh.Metadata.Annotations == nil {
		fresh.Metadata.Annotations = map[string]string{}
	}
	fresh.Metadata.Annotations[RevisionAnnotation] = want
	*d = *fresh
	return s.Put(fresh)
}

// cleanupHistory deletes the oldest scaled-down ReplicaSets beyond revisionHistoryLimit.
func cleanupHistory(s *State, d *Deployment, old []*ReplicaSet) error {
	limit := int32(10)
	if d.Spec.RevisionHistoryLimit != nil {
		limit = *d.Spec.RevisionHistoryLimit
	}
	var idle []*ReplicaSet
	for _, rs := range old {
		if rs.DesiredReplicas() == 0 && rs.Status.Replicas == 0 {
			idle = append(idle, rs)
		}
	}
	for i := 0; i < len(idle)-int(limit); i++ {
		s.Delete(KindReplicaSet, idle[i].Metadata.Namespace, idle[i].Metadata.Name)
	}
	return nil
}

func updateDeploymentStatus(s *State, ns, name, hash string, desired, maxUnavailable int32) error {
	d := s.Get(KindDeployment, ns, name).(*Deployment)
	var st DeploymentStatus
	for _, o := range ownedBy(s, KindReplicaSet, ns, d.Metadata.UID) {
		rs := o.(*ReplicaSet)
		st.Replicas += rs.Status.Replicas
		st.ReadyReplicas += rs.Status.ReadyReplicas
		st.AvailableReplicas += rs.Status.AvailableReplicas
		if rs.Metadata.Labels[PodTemplateHashLabel] == hash {
			st.UpdatedReplicas = rs.Status.Replicas
		}
	}
	st.ObservedGeneration = d.Metadata.Generation
	st.UnavailableReplicas = max(desired-st.AvailableReplicas, 0)
	available := "False"
	availReason, availMsg := "MinimumReplicasUnavailable", "Deployment does not have minimum availability."
	if st.AvailableReplicas >= desired-maxUnavailable {
		available, availReason, availMsg = "True", "MinimumReplicasAvailable", "Deployment has minimum availability."
	}
	progReason, progMsg := "ReplicaSetUpdated", fmt.Sprintf("ReplicaSet %q is progressing.", d.Metadata.Name+"-"+hash)
	if st.UpdatedReplicas == desired && st.Replicas == desired && st.AvailableReplicas == desired {
		progReason, progMsg = "NewReplicaSetAvailable", fmt.Sprintf("ReplicaSet %q has successfully progressed.", d.Metadata.Name+"-"+hash)
	}
	st.Conditions = []DeploymentCondition{
		{Type: "Available", Status: available, Reason: availReason, Message: availMsg},
		{Type: "Progressing", Status: "True", Reason: progReason, Message: progMsg},
	}
	if jsonEqual(d.Status, st) {
		return nil
	}
	d.Status = st
	return s.Put(d)
}

// ---- ReplicaSet controller ----

func reconcileReplicaSets(s *State) error {
	for _, o := range s.List(KindReplicaSet, "*") {
		if err := reconcileReplicaSet(s, o.(*ReplicaSet)); err != nil {
			return err
		}
	}
	return nil
}

func reconcileReplicaSet(s *State, rs *ReplicaSet) error {
	ns := rs.Metadata.Namespace
	sel, err := SelectorFromLabelSelector(rs.Spec.Selector)
	if err != nil {
		return err
	}
	var pods []*Pod
	for _, o := range ownedBy(s, KindPod, ns, rs.Metadata.UID) {
		p := o.(*Pod)
		if sel.Matches(p.Metadata.Labels) {
			pods = append(pods, p)
		}
	}
	desired := int(rs.DesiredReplicas())
	if desired > s.Limits.MaxPodsPerOwner {
		return fmt.Errorf("%w: at most %d Pods per ReplicaSet", ErrLimit, s.Limits.MaxPodsPerOwner)
	}
	for len(pods) < desired {
		name := ""
		for {
			name = rs.Metadata.Name + "-" + s.Gen.Suffix()
			if !s.Has(KindPod, ns, name) {
				break
			}
		}
		p := &Pod{
			Metadata: ObjectMeta{
				Name: name, Namespace: ns, GenerateName: rs.Metadata.Name + "-",
				Labels:          copyLabels(rs.Spec.Template.Metadata.Labels),
				Annotations:     copyLabels(rs.Spec.Template.Metadata.Annotations),
				OwnerReferences: []OwnerReference{controllerRef(rs)},
			},
			Spec: rs.Spec.Template.Spec,
		}
		if len(p.Metadata.Annotations) == 0 {
			p.Metadata.Annotations = nil
		}
		p.Spec.RestartPolicy = "Always"
		p.Status.Phase = "Pending"
		if err := s.Put(p); err != nil {
			return err
		}
		s.Record("Normal", "SuccessfulCreate", rs, "Created pod: "+name)
		pods = append(pods, s.Get(KindPod, ns, name).(*Pod))
	}
	if len(pods) > desired {
		// Delete unscheduled, then not-ready, then newest Pods first (as the real controller ranks).
		sort.SliceStable(pods, func(i, j int) bool {
			a, b := pods[i], pods[j]
			if (a.Spec.NodeName == "") != (b.Spec.NodeName == "") {
				return a.Spec.NodeName == ""
			}
			if a.Ready() != b.Ready() {
				return !a.Ready()
			}
			if !a.Metadata.CreationTimestamp.Equal(b.Metadata.CreationTimestamp) {
				return a.Metadata.CreationTimestamp.After(b.Metadata.CreationTimestamp)
			}
			return a.Metadata.Name > b.Metadata.Name
		})
		excess := len(pods) - desired
		for _, p := range pods[:excess] {
			s.Delete(KindPod, ns, p.Metadata.Name)
			s.Record("Normal", "SuccessfulDelete", rs, "Deleted pod: "+p.Metadata.Name)
		}
		pods = pods[excess:]
	}

	st := ReplicaSetStatus{ObservedGeneration: rs.Metadata.Generation}
	for _, p := range pods {
		st.Replicas++
		st.FullyLabeledReplicas++
		if p.Ready() {
			st.ReadyReplicas++
			st.AvailableReplicas++
		}
	}
	fresh := s.Get(KindReplicaSet, ns, rs.Metadata.Name).(*ReplicaSet)
	if jsonEqual(fresh.Status, st) {
		return nil
	}
	fresh.Status = st
	return s.Put(fresh)
}
