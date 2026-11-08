package simulator

import (
	"errors"
	"testing"
)

func writeTestState(t *testing.T) *State {
	t.Helper()
	s := NewState(DefaultEpoch, 1, NewImageCatalog(nil))
	for _, ns := range SystemNamespaces {
		if err := s.Put(&Namespace{Metadata: ObjectMeta{Name: ns}}); err != nil {
			t.Fatal(err)
		}
	}
	node := &Node{Metadata: ObjectMeta{Name: "node-1"}}
	node.Status.Allocatable = map[string]string{"cpu": "4", "memory": "8Gi", "pods": "110"}
	node.Status.Conditions = []NodeCondition{{Type: "Ready", Status: "True"}}
	if err := s.Put(node); err != nil {
		t.Fatal(err)
	}
	return s
}

func newTestDeployment(name string, replicas int32) *Deployment {
	labels := map[string]string{"app": name}
	return &Deployment{
		Metadata: ObjectMeta{Name: name, Namespace: "default"},
		Spec: DeploymentSpec{
			Replicas: &replicas, Selector: LabelSelector{MatchLabels: labels},
			Template: PodTemplateSpec{Metadata: TemplateMeta{Labels: labels}, Spec: PodSpec{Containers: []Container{{Name: "web", Image: "nginx:1.27"}}}},
		},
	}
}

func TestCreateChecksLikeTheAPIServer(t *testing.T) {
	s := writeTestState(t)
	var notFound *NotFoundError
	cm := &ConfigMap{Metadata: ObjectMeta{Name: "c", Namespace: "missing"}}
	if err := s.Create(cm); !errors.As(err, &notFound) || notFound.Kind != KindNamespace {
		t.Fatalf("missing namespace: %v", err)
	}
	cm.Metadata.Namespace = "default"
	if err := s.Create(cm.DeepCopy()); err != nil {
		t.Fatal(err)
	}
	var exists *AlreadyExistsError
	if err := s.Create(cm.DeepCopy()); !errors.As(err, &exists) {
		t.Fatalf("duplicate: %v", err)
	}
	var forbidden *ForbiddenError
	if err := s.Create(&Node{Metadata: ObjectMeta{Name: "node-9"}}); !errors.As(err, &forbidden) {
		t.Fatalf("node: %v", err)
	}
	var invalid *ValidationError
	if err := s.Create(&Pod{Metadata: ObjectMeta{Name: "p", Namespace: "default"}}); !errors.As(err, &invalid) {
		t.Fatalf("pod without containers: %v", err)
	}
	// Server-owned metadata in the input is replaced.
	ns := &Namespace{Metadata: ObjectMeta{Name: "dev", UID: "chosen-by-learner", ResourceVersion: "999"}}
	if err := s.Create(ns); err != nil {
		t.Fatal(err)
	}
	got := s.Get(KindNamespace, "", "dev").(*Namespace)
	if got.Metadata.UID == "chosen-by-learner" || got.Metadata.ResourceVersion == "999" || got.Status.Phase != "Active" {
		t.Fatalf("namespace %+v", got.Metadata)
	}
}

func TestUpdateKeepsImmutableFieldsAndStatus(t *testing.T) {
	s := writeTestState(t)
	if err := s.Create(newTestDeployment("web", 2)); err != nil {
		t.Fatal(err)
	}
	if _, err := Reconcile(s); err != nil {
		t.Fatal(err)
	}
	d := s.Get(KindDeployment, "default", "web").(*Deployment)
	ready := d.Status.ReadyReplicas
	d.Spec.Selector.MatchLabels = map[string]string{"app": "other"}
	d.Spec.Template.Metadata.Labels = map[string]string{"app": "other"}
	var invalid *ValidationError
	if err := s.Update(d); !errors.As(err, &invalid) {
		t.Fatalf("selector change: %v", err)
	}
	d = s.Get(KindDeployment, "default", "web").(*Deployment)
	d.Status = DeploymentStatus{} // a learner can't change status
	d.Metadata.Labels = map[string]string{"team": "a"}
	if err := s.Update(d); err != nil {
		t.Fatal(err)
	}
	if got := s.Get(KindDeployment, "default", "web").(*Deployment); got.Status.ReadyReplicas != ready || got.Metadata.Labels["team"] != "a" {
		t.Fatalf("status %+v labels %v", got.Status, got.Metadata.Labels)
	}
	// Nodes accept label changes only.
	n := s.Get(KindNode, "", "node-1").(*Node)
	n.Metadata.Labels = map[string]string{"disktype": "ssd"}
	n.Spec.Unschedulable = true
	if err := s.Update(n); err != nil {
		t.Fatal(err)
	}
	if got := s.Get(KindNode, "", "node-1").(*Node); got.Metadata.Labels["disktype"] != "ssd" || got.Spec.Unschedulable {
		t.Fatalf("node %+v", got)
	}
}

func TestNodePortsAreNotShared(t *testing.T) {
	s := writeTestState(t)
	svc := func(name string) *Service {
		return &Service{Metadata: ObjectMeta{Name: name, Namespace: "default"}, Spec: ServiceSpec{
			Type: "NodePort", Ports: []ServicePort{{Port: 80, NodePort: 30080}},
		}}
	}
	if err := s.Create(svc("a")); err != nil {
		t.Fatal(err)
	}
	var invalid *ValidationError
	if err := s.Create(svc("b")); !errors.As(err, &invalid) {
		t.Fatalf("shared node port: %v", err)
	}
	// Updating the Service that holds the port is fine.
	a := s.Get(KindService, "default", "a").(*Service)
	a.Metadata.Labels = map[string]string{"x": "y"}
	if err := s.Update(a); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveCascadesOrOrphans(t *testing.T) {
	for _, orphan := range []bool{false, true} {
		s := writeTestState(t)
		if err := s.Create(newTestDeployment("web", 2)); err != nil {
			t.Fatal(err)
		}
		if _, err := Reconcile(s); err != nil {
			t.Fatal(err)
		}
		if err := s.Remove(KindDeployment, "default", "web", orphan); err != nil {
			t.Fatal(err)
		}
		if _, err := Reconcile(s); err != nil {
			t.Fatal(err)
		}
		rs, pods := s.List(KindReplicaSet, "default"), s.List(KindPod, "default")
		if orphan && (len(rs) != 1 || len(pods) != 2 || ControllerOf(rs[0]) != "") {
			t.Fatalf("orphan: %d ReplicaSets (%v), %d Pods", len(rs), rs, len(pods))
		}
		if !orphan && (len(rs) != 0 || len(pods) != 0) {
			t.Fatalf("cascade left %d ReplicaSets and %d Pods", len(rs), len(pods))
		}
		if orphan {
			// A new Deployment with a matching selector adopts the orphaned ReplicaSet.
			if err := s.Create(newTestDeployment("web", 2)); err != nil {
				t.Fatal(err)
			}
			if _, err := Reconcile(s); err != nil {
				t.Fatal(err)
			}
			d := s.Get(KindDeployment, "default", "web")
			if rs := s.List(KindReplicaSet, "default"); len(rs) != 1 || ControllerOf(rs[0]) != d.Meta().UID {
				t.Fatalf("adoption: %v", rs)
			}
			if pods := s.List(KindPod, "default"); len(pods) != 2 {
				t.Fatalf("adoption recreated Pods: %d", len(pods))
			}
		}
	}
	s := writeTestState(t)
	var forbidden *ForbiddenError
	if err := s.Remove(KindNamespace, "", "kube-system", false); !errors.As(err, &forbidden) {
		t.Fatalf("system namespace: %v", err)
	}
}

func TestReplicaSetReleasesAndAdoptsPods(t *testing.T) {
	s := writeTestState(t)
	if err := s.Create(newTestDeployment("web", 1)); err != nil {
		t.Fatal(err)
	}
	if _, err := Reconcile(s); err != nil {
		t.Fatal(err)
	}
	pod := s.List(KindPod, "default")[0].(*Pod)
	pod.Metadata.Labels["app"] = "debug"
	if err := s.Update(pod); err != nil {
		t.Fatal(err)
	}
	if _, err := Reconcile(s); err != nil {
		t.Fatal(err)
	}
	pods := s.List(KindPod, "default")
	if len(pods) != 2 {
		t.Fatalf("want the released Pod plus a replacement, got %d", len(pods))
	}
	released := s.Get(KindPod, "default", pod.Metadata.Name)
	if ControllerOf(released) != "" {
		t.Fatal("the relabelled Pod is still controlled by its ReplicaSet")
	}
	// Putting the label back makes the ReplicaSet adopt it and remove the extra Pod.
	back := released.(*Pod)
	back.Metadata.Labels["app"] = "web"
	if err := s.Update(back); err != nil {
		t.Fatal(err)
	}
	if _, err := Reconcile(s); err != nil {
		t.Fatal(err)
	}
	if pods := s.List(KindPod, "default"); len(pods) != 1 {
		t.Fatalf("after adoption want 1 Pod, got %d", len(pods))
	}
}

func TestCloneIsIndependent(t *testing.T) {
	s := writeTestState(t)
	c := s.Clone()
	if err := c.Create(&ConfigMap{Metadata: ObjectMeta{Name: "x", Namespace: "default"}}); err != nil {
		t.Fatal(err)
	}
	c.Advance(Tick)
	if s.Has(KindConfigMap, "default", "x") || !s.Now.Equal(DefaultEpoch) || len(s.changed) == len(c.changed) {
		t.Fatal("changes to the clone leaked into the original")
	}
}
