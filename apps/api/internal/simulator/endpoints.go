package simulator

const serviceNameLabel = "kubernetes.io/service-name"

// reconcileEndpointSlices keeps one EndpointSlice per Service with a selector, listing the
// matching Pods that have an IP (ready or not, with their readiness).
func reconcileEndpointSlices(s *State) error {
	services := map[string]*Service{}
	for _, o := range s.List(KindService, "*") {
		svc := o.(*Service)
		services[svc.Metadata.UID] = svc
		if len(svc.Spec.Selector) == 0 {
			continue
		}
		if err := syncSlice(s, svc); err != nil {
			return err
		}
	}
	// Slices of Services that lost their selector are removed (deleted Services are handled by GC).
	for _, o := range s.List(KindEndpointSlice, "*") {
		owner := ControllerOf(o)
		if svc, ok := services[owner]; ok && len(svc.Spec.Selector) == 0 {
			s.Delete(KindEndpointSlice, o.Meta().Namespace, o.Meta().Name)
		}
	}
	return nil
}

func syncSlice(s *State, svc *Service) error {
	ns := svc.Metadata.Namespace
	var slice *EndpointSlice
	for _, o := range ownedBy(s, KindEndpointSlice, ns, svc.Metadata.UID) {
		slice = o.(*EndpointSlice)
		break
	}
	if slice == nil {
		name := ""
		for {
			name = svc.Metadata.Name + "-" + s.Gen.Suffix()
			if !s.Has(KindEndpointSlice, ns, name) {
				break
			}
		}
		slice = &EndpointSlice{
			Metadata: ObjectMeta{
				Name: name, Namespace: ns, GenerateName: svc.Metadata.Name + "-",
				Labels: map[string]string{
					serviceNameLabel:                         svc.Metadata.Name,
					"endpointslice.kubernetes.io/managed-by": "endpointslice-controller.k8s.io",
				},
				OwnerReferences: []OwnerReference{controllerRef(svc)},
			},
			AddressType: "IPv4",
		}
	}

	sel := SelectorFromLabels(svc.Spec.Selector)
	endpoints := []Endpoint{}
	var firstPod *Pod
	for _, o := range s.List(KindPod, ns) {
		p := o.(*Pod)
		if !sel.Matches(p.Metadata.Labels) || p.Status.PodIP == "" {
			continue
		}
		if firstPod == nil {
			firstPod = p
		}
		ready := p.Ready()
		endpoints = append(endpoints, Endpoint{
			Addresses:  []string{p.Status.PodIP},
			Conditions: EndpointConditions{Ready: ready, Serving: ready},
			NodeName:   p.Spec.NodeName,
			TargetRef:  &ObjectReference{Kind: KindPod, Namespace: ns, Name: p.Metadata.Name, UID: p.Metadata.UID},
		})
	}
	ports := []EndpointPort{}
	for _, sp := range svc.Spec.Ports {
		port := sp.TargetPort.IntVal
		if sp.TargetPort.IsStr && firstPod != nil {
			port = namedPort(firstPod, sp.TargetPort.StrVal)
		}
		if port == 0 {
			port = sp.Port
		}
		proto := sp.Protocol
		if proto == "" {
			proto = "TCP"
		}
		ports = append(ports, EndpointPort{Name: sp.Name, Port: port, Protocol: proto})
	}
	if jsonEqual(slice.Endpoints, endpoints) && jsonEqual(slice.Ports, ports) && slice.Metadata.ResourceVersion != "" {
		return nil
	}
	slice.Endpoints, slice.Ports = endpoints, ports
	return s.Put(slice)
}

func namedPort(p *Pod, name string) int32 {
	for _, c := range p.Spec.Containers {
		for _, port := range c.Ports {
			if port.Name == name {
				return port.ContainerPort
			}
		}
	}
	return 0
}
