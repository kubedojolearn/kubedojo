package simulator

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	dns1123Label     = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	dns1123Subdomain = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	labelName        = regexp.MustCompile(`^([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]$`)
	labelValue       = regexp.MustCompile(`^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$`)
	envVarName       = regexp.MustCompile(`^[-._a-zA-Z][-._a-zA-Z0-9]*$`)
	configKey        = regexp.MustCompile(`^[-._a-zA-Z0-9]+$`)
	imageRef         = regexp.MustCompile(`^[a-z0-9]+([._/-][a-z0-9]+)*(:[A-Za-z0-9_][A-Za-z0-9_.-]{0,127})?(@sha256:[a-f0-9]{64})?$`)
)

// ValidationError is a learner-facing validation failure, worded like the API server's.
type ValidationError struct {
	Kind, Name string
	Reasons    []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s %q is invalid: %s", e.Kind, e.Name, strings.Join(e.Reasons, "; "))
}

// ValidateName checks an object name for its kind.
func ValidateName(kind, name string) error {
	if name == "" {
		return fmt.Errorf("metadata.name: Required value")
	}
	if kind == KindNamespace || kind == KindService {
		if len(name) > 63 || !dns1123Label.MatchString(name) {
			return fmt.Errorf("metadata.name: Invalid value: %q: a lowercase RFC 1123 label must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character", name)
		}
		return nil
	}
	if len(name) > 253 || !dns1123Subdomain.MatchString(name) {
		return fmt.Errorf("metadata.name: Invalid value: %q: a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character", name)
	}
	return nil
}

// ValidateLabelKey checks a label or annotation key ([prefix/]name).
func ValidateLabelKey(key string) error {
	prefix, name, hasPrefix := strings.Cut(key, "/")
	if !hasPrefix {
		name, prefix = prefix, ""
	}
	if hasPrefix && (prefix == "" || len(prefix) > 253 || !dns1123Subdomain.MatchString(prefix)) {
		return fmt.Errorf("invalid label key %q: prefix must be a DNS subdomain", key)
	}
	if name == "" || len(name) > 63 || !labelName.MatchString(name) {
		return fmt.Errorf("invalid label key %q: name part must be 63 characters or less, beginning and ending with an alphanumeric character", key)
	}
	return nil
}

// ValidateLabels checks label keys and values.
func ValidateLabels(labels map[string]string) []string {
	var reasons []string
	for _, k := range sortedKeys(labels) {
		if err := ValidateLabelKey(k); err != nil {
			reasons = append(reasons, "metadata.labels: "+err.Error())
		}
		if v := labels[k]; len(v) > 63 || !labelValue.MatchString(v) {
			reasons = append(reasons, fmt.Sprintf("metadata.labels: Invalid value: %q: a valid label value must be 63 characters or less", v))
		}
	}
	return reasons
}

func validateAnnotations(a map[string]string) []string {
	var reasons []string
	total := 0
	for _, k := range sortedKeys(a) {
		if err := ValidateLabelKey(k); err != nil {
			reasons = append(reasons, "metadata.annotations: "+err.Error())
		}
		total += len(k) + len(a[k])
	}
	if total > 256*1024 {
		reasons = append(reasons, "metadata.annotations: Too long: must have at most 262144 bytes")
	}
	return reasons
}

func validateContainers(containers []Container, field string) []string {
	var reasons []string
	if len(containers) == 0 {
		return []string{field + ": Required value"}
	}
	seen := map[string]bool{}
	for i, c := range containers {
		f := fmt.Sprintf("%s[%d]", field, i)
		if c.Name == "" || len(c.Name) > 63 || !dns1123Label.MatchString(c.Name) {
			reasons = append(reasons, f+".name: Invalid value: "+fmt.Sprintf("%q", c.Name))
		}
		if seen[c.Name] {
			reasons = append(reasons, f+".name: Duplicate value: "+fmt.Sprintf("%q", c.Name))
		}
		seen[c.Name] = true
		if c.Image == "" {
			reasons = append(reasons, f+".image: Required value")
		} else if !imageRef.MatchString(c.Image) {
			reasons = append(reasons, f+".image: Invalid value: "+fmt.Sprintf("%q", c.Image))
		}
		for _, e := range c.Env {
			if !envVarName.MatchString(e.Name) {
				reasons = append(reasons, f+".env.name: Invalid value: "+fmt.Sprintf("%q", e.Name))
			}
			if e.ValueFrom != nil && (e.ValueFrom.ConfigMapKeyRef == nil) == (e.ValueFrom.SecretKeyRef == nil) {
				reasons = append(reasons, f+".env.valueFrom: Invalid value: exactly one of configMapKeyRef or secretKeyRef is required")
			}
		}
		for _, p := range c.Ports {
			if p.ContainerPort < 1 || p.ContainerPort > 65535 {
				reasons = append(reasons, f+".ports.containerPort: Invalid value: must be between 1 and 65535")
			}
		}
		for res, q := range c.Resources.Requests {
			reasons = append(reasons, validateQuantity(f+".resources.requests", res, q)...)
		}
		for res, q := range c.Resources.Limits {
			reasons = append(reasons, validateQuantity(f+".resources.limits", res, q)...)
		}
		reasons = append(reasons, validateRequestsWithinLimits(f, c.Resources)...)
	}
	return reasons
}

func validateQuantity(field, res, q string) []string {
	var err error
	switch res {
	case "cpu":
		_, err = ParseCPU(q)
	case "memory":
		_, err = ParseMemory(q)
	default:
		return []string{fmt.Sprintf("%s[%s]: Unsupported resource (KubeDojo simulates cpu and memory)", field, res)}
	}
	if err != nil {
		return []string{fmt.Sprintf("%s[%s]: Invalid value: %q", field, res, q)}
	}
	return nil
}

func validateRequestsWithinLimits(field string, r ResourceRequirements) []string {
	var reasons []string
	if req, ok := r.Requests["cpu"]; ok {
		if lim, ok := r.Limits["cpu"]; ok {
			a, e1 := ParseCPU(req)
			b, e2 := ParseCPU(lim)
			if e1 == nil && e2 == nil && a > b {
				reasons = append(reasons, field+".resources.requests: Invalid value: "+fmt.Sprintf("%q", req)+": must be less than or equal to cpu limit")
			}
		}
	}
	if req, ok := r.Requests["memory"]; ok {
		if lim, ok := r.Limits["memory"]; ok {
			a, e1 := ParseMemory(req)
			b, e2 := ParseMemory(lim)
			if e1 == nil && e2 == nil && a > b {
				reasons = append(reasons, field+".resources.requests: Invalid value: "+fmt.Sprintf("%q", req)+": must be less than or equal to memory limit")
			}
		}
	}
	return reasons
}

// Validate checks an object against the supported subset. It does not check cross-object
// rules (existence of namespaces), which the State does.
func Validate(o Object) error {
	m := o.Meta()
	kind := o.GetKind()
	var reasons []string
	if err := ValidateName(kind, m.Name); err != nil {
		reasons = append(reasons, err.Error())
	}
	if ClusterScoped(kind) && m.Namespace != "" {
		reasons = append(reasons, "metadata.namespace: Forbidden: "+kind+" is cluster-scoped")
	}
	if !ClusterScoped(kind) && (m.Namespace == "" || !dns1123Label.MatchString(m.Namespace)) {
		reasons = append(reasons, "metadata.namespace: Invalid value: "+fmt.Sprintf("%q", m.Namespace))
	}
	reasons = append(reasons, ValidateLabels(m.Labels)...)
	reasons = append(reasons, validateAnnotations(m.Annotations)...)

	switch v := o.(type) {
	case *Pod:
		reasons = append(reasons, validateContainers(v.Spec.Containers, "spec.containers")...)
		if rp := v.Spec.RestartPolicy; rp != "" && rp != "Always" {
			reasons = append(reasons, "spec.restartPolicy: Unsupported value: "+fmt.Sprintf("%q", rp)+" (KubeDojo simulates Always)")
		}
		reasons = append(reasons, ValidateLabels(v.Spec.NodeSelector)...)
	case *Deployment:
		reasons = append(reasons, validateWorkload(v.Spec.Replicas, v.Spec.Selector, v.Spec.Template)...)
		switch v.Spec.Strategy.Type {
		case "", "RollingUpdate", "Recreate":
		default:
			reasons = append(reasons, "spec.strategy.type: Unsupported value: "+fmt.Sprintf("%q", v.Spec.Strategy.Type))
		}
	case *ReplicaSet:
		reasons = append(reasons, validateWorkload(v.Spec.Replicas, v.Spec.Selector, v.Spec.Template)...)
	case *Service:
		switch v.Spec.Type {
		case "", "ClusterIP", "NodePort", "LoadBalancer":
		default:
			reasons = append(reasons, "spec.type: Unsupported value: "+fmt.Sprintf("%q", v.Spec.Type)+" (KubeDojo simulates ClusterIP, NodePort and LoadBalancer)")
		}
		if len(v.Spec.Ports) == 0 {
			reasons = append(reasons, "spec.ports: Required value")
		}
		for i, p := range v.Spec.Ports {
			if p.Port < 1 || p.Port > 65535 {
				reasons = append(reasons, fmt.Sprintf("spec.ports[%d].port: Invalid value: %d", i, p.Port))
			}
			if p.NodePort != 0 && (p.NodePort < 30000 || p.NodePort > 32767) {
				reasons = append(reasons, fmt.Sprintf("spec.ports[%d].nodePort: Invalid value: %d: provided port is not in the valid range. The range of valid ports is 30000-32767", i, p.NodePort))
			}
			if len(v.Spec.Ports) > 1 && p.Name == "" {
				reasons = append(reasons, fmt.Sprintf("spec.ports[%d].name: Required value", i))
			}
		}
		reasons = append(reasons, ValidateLabels(v.Spec.Selector)...)
	case *ConfigMap:
		size := 0
		for k, val := range v.Data {
			if !configKey.MatchString(k) {
				reasons = append(reasons, "data["+k+"]: Invalid value: a valid config key must consist of alphanumeric characters, '-', '_' or '.'")
			}
			size += len(k) + len(val)
		}
		if size > 64*1024 {
			reasons = append(reasons, "data: Too long: KubeDojo limits ConfigMaps to 65536 bytes")
		}
	case *Secret:
		if v.Type != "" && v.Type != "Opaque" {
			reasons = append(reasons, "type: Unsupported value: "+fmt.Sprintf("%q", v.Type)+" (KubeDojo simulates Opaque Secrets)")
		}
	}
	if len(reasons) > 0 {
		return &ValidationError{Kind: kind, Name: m.Name, Reasons: reasons}
	}
	return nil
}

func validateWorkload(replicas *int32, sel LabelSelector, tpl PodTemplateSpec) []string {
	var reasons []string
	if replicas != nil && (*replicas < 0 || *replicas > 50) {
		reasons = append(reasons, fmt.Sprintf("spec.replicas: Invalid value: %d: KubeDojo allows 0 to 50 replicas", *replicas))
	}
	if len(sel.MatchLabels) == 0 && len(sel.MatchExpressions) == 0 {
		reasons = append(reasons, "spec.selector: Required value")
	}
	s, err := SelectorFromLabelSelector(sel)
	if err != nil {
		reasons = append(reasons, "spec.selector: "+err.Error())
	} else if !s.Matches(tpl.Metadata.Labels) {
		reasons = append(reasons, "spec.template.metadata.labels: Invalid value: `selector` does not match template `labels`")
	}
	reasons = append(reasons, ValidateLabels(tpl.Metadata.Labels)...)
	reasons = append(reasons, validateContainers(tpl.Spec.Containers, "spec.template.spec.containers")...)
	return reasons
}

// ValidateUpdate enforces immutable fields when replacing old with updated.
func ValidateUpdate(old, updated Object) error {
	var reasons []string
	switch o := old.(type) {
	case *Pod:
		n := updated.(*Pod)
		a, b := o.DeepCopy().(*Pod), n.DeepCopy().(*Pod)
		a.Spec.NodeName, b.Spec.NodeName = "", ""
		if !jsonEqual(a.Spec, b.Spec) {
			reasons = append(reasons, "spec: Forbidden: pod updates may not change fields other than labels and annotations in KubeDojo")
		}
	case *Deployment:
		if !jsonEqual(o.Spec.Selector, updated.(*Deployment).Spec.Selector) {
			reasons = append(reasons, "spec.selector: Invalid value: field is immutable")
		}
	case *ReplicaSet:
		if !jsonEqual(o.Spec.Selector, updated.(*ReplicaSet).Spec.Selector) {
			reasons = append(reasons, "spec.selector: Invalid value: field is immutable")
		}
	case *Service:
		n := updated.(*Service)
		if n.Spec.ClusterIP != "" && n.Spec.ClusterIP != o.Spec.ClusterIP {
			reasons = append(reasons, "spec.clusterIP: Invalid value: field is immutable")
		}
	}
	if len(reasons) > 0 {
		return &ValidationError{Kind: old.GetKind(), Name: old.Meta().Name, Reasons: reasons}
	}
	return nil
}
