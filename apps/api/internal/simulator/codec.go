package simulator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// NewObject returns an empty object of a supported kind.
func NewObject(kind string) (Object, error) {
	var o Object
	switch kind {
	case KindNamespace:
		o = &Namespace{}
	case KindNode:
		o = &Node{}
	case KindPod:
		o = &Pod{}
	case KindDeployment:
		o = &Deployment{}
	case KindReplicaSet:
		o = &ReplicaSet{}
	case KindService:
		o = &Service{}
	case KindEndpointSlice:
		o = &EndpointSlice{}
	case KindConfigMap:
		o = &ConfigMap{}
	case KindSecret:
		o = &Secret{}
	default:
		return nil, &UnsupportedError{What: fmt.Sprintf("kind %q", kind), Hint: "see the simulator coverage page for supported kinds"}
	}
	TypeMetaFor(o)
	return o, nil
}

// UnsupportedError reports input outside the simulated subset (never silently ignored).
type UnsupportedError struct {
	What string
	Hint string
}

func (e *UnsupportedError) Error() string {
	msg := e.What + " is not supported by the KubeDojo simulator"
	if e.Hint != "" {
		msg += " (" + e.Hint + ")"
	}
	return msg
}

// Stored is an object split into the columns of lab_resources.
type Stored struct {
	Kind, Namespace, Name, APIVersion, UID string
	ResourceVersion, Generation            int64
	Metadata, Spec, Status, Extra          json.RawMessage
}

// Encode splits an object into stored columns. Kind-specific top-level fields (ConfigMap data,
// Secret type/data, EndpointSlice endpoints/ports/addressType) go into Extra.
func Encode(o Object) (Stored, error) {
	raw, err := json.Marshal(o)
	if err != nil {
		return Stored{}, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Stored{}, err
	}
	m := o.Meta()
	st := Stored{
		Kind: o.GetKind(), Namespace: m.Namespace, Name: m.Name, APIVersion: APIVersion(o.GetKind()),
		UID: m.UID, Generation: max(m.Generation, 1),
		Metadata: fields["metadata"], Spec: orEmptyObject(fields["spec"]), Status: orEmptyObject(fields["status"]),
	}
	if _, err := fmt.Sscan(m.ResourceVersion, &st.ResourceVersion); err != nil {
		return Stored{}, fmt.Errorf("object %s has no resource version", KeyOf(o))
	}
	for _, k := range []string{"apiVersion", "kind", "metadata", "spec", "status"} {
		delete(fields, k)
	}
	extra, err := json.Marshal(fields)
	if err != nil {
		return Stored{}, err
	}
	st.Extra = extra
	return st, nil
}

func orEmptyObject(b json.RawMessage) json.RawMessage {
	if len(b) == 0 || string(b) == "null" {
		return json.RawMessage(`{}`)
	}
	return b
}

// DecodeStored rebuilds an object from stored columns.
func DecodeStored(st Stored) (Object, error) {
	o, err := NewObject(st.Kind)
	if err != nil {
		return nil, err
	}
	fields := map[string]json.RawMessage{}
	if len(st.Extra) > 0 {
		if err := json.Unmarshal(st.Extra, &fields); err != nil {
			return nil, err
		}
	}
	fields["metadata"] = st.Metadata
	if string(st.Spec) != "{}" {
		fields["spec"] = st.Spec
	}
	if string(st.Status) != "{}" {
		fields["status"] = st.Status
	}
	raw, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, o); err != nil {
		return nil, fmt.Errorf("decode stored %s: %w", st.Kind, err)
	}
	TypeMetaFor(o)
	return o, nil
}

// DecodeInput decodes a user- or fixture-supplied object. apiVersion and kind must match a
// supported kind, and unknown fields are rejected as unsupported rather than dropped. Secret
// stringData is merged into data, as the API server does.
func DecodeInput(raw []byte) (Object, error) {
	var head struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &head); err != nil {
		return nil, fmt.Errorf("invalid object: %w", err)
	}
	if head.Kind == "" || head.APIVersion == "" {
		return nil, fmt.Errorf("object is missing apiVersion or kind")
	}
	o, err := NewObject(head.Kind)
	if err != nil {
		return nil, err
	}
	if want := APIVersion(head.Kind); head.APIVersion != want {
		return nil, &UnsupportedError{What: fmt.Sprintf("apiVersion %q for %s", head.APIVersion, head.Kind), Hint: "use " + want}
	}

	var stringData map[string]string
	if head.Kind == KindSecret {
		var generic map[string]json.RawMessage
		if err := json.Unmarshal(raw, &generic); err == nil {
			if sd, ok := generic["stringData"]; ok {
				if err := json.Unmarshal(sd, &stringData); err != nil {
					return nil, fmt.Errorf("stringData: %w", err)
				}
				delete(generic, "stringData")
				raw, _ = json.Marshal(generic)
			}
		}
	}
	// Status is owned by the simulator: accept it in input but ignore it, like kubectl apply.
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(raw, &generic); err == nil {
		if _, ok := generic["status"]; ok && head.Kind != KindNode {
			delete(generic, "status")
			raw, _ = json.Marshal(generic)
		}
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(o); err != nil {
		if field, ok := strings.CutPrefix(err.Error(), "json: unknown field "); ok {
			return nil, &UnsupportedError{What: "field " + strings.Trim(field, `"`) + " in " + head.Kind}
		}
		return nil, fmt.Errorf("invalid %s: %w", head.Kind, err)
	}
	if s, ok := o.(*Secret); ok {
		if s.Type == "" {
			s.Type = "Opaque"
		}
		for k, v := range stringData {
			if s.Data == nil {
				s.Data = map[string][]byte{}
			}
			s.Data[k] = []byte(v)
		}
	}
	TypeMetaFor(o)
	return o, nil
}

// MarshalForOutput renders an object as kubectl -o json would, with the simulated annotation.
func MarshalForOutput(o Object) ([]byte, error) {
	c := o.DeepCopy()
	m := c.Meta()
	if m.Annotations == nil {
		m.Annotations = map[string]string{}
	}
	m.Annotations[SimulatedAnnotation] = "true"
	return json.MarshalIndent(c, "", "    ")
}
