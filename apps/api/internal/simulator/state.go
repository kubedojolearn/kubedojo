package simulator

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"time"
)

// Key identifies an object. Cluster-scoped objects use Namespace "".
type Key struct {
	Kind      string
	Namespace string
	Name      string
}

func (k Key) String() string {
	if k.Namespace == "" {
		return k.Kind + "/" + k.Name
	}
	return k.Kind + "/" + k.Namespace + "/" + k.Name
}

// KeyOf returns the key of an object.
func KeyOf(o Object) Key {
	m := o.Meta()
	return Key{Kind: o.GetKind(), Namespace: m.Namespace, Name: m.Name}
}

// Limits bound what one lab may contain (simulator-compatibility.md §5).
type Limits struct {
	MaxObjects      int
	MaxEvents       int
	MaxPodsPerOwner int
}

// DefaultLimits are the documented defaults.
var DefaultLimits = Limits{MaxObjects: 100, MaxEvents: 500, MaxPodsPerOwner: 50}

// ErrLimit is returned when an operation would exceed a lab limit.
var ErrLimit = errors.New("lab limit reached")

// Generator is the lab's deterministic source of IDs: splitmix64 over (seed, counter).
type Generator struct {
	Seed    uint64
	Counter uint64
}

func (g *Generator) next() uint64 {
	g.Counter++
	z := g.Seed + g.Counter*0x9E3779B97F4A7C15
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// UID returns a UUID-formatted deterministic identifier.
func (g *Generator) UID() string {
	var u [16]byte
	binary.BigEndian.PutUint64(u[:8], g.next())
	binary.BigEndian.PutUint64(u[8:], g.next())
	u[6] = (u[6] & 0x0f) | 0x40
	u[8] = (u[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])
}

// nameAlphabet is the alphabet Kubernetes uses for generated names (no vowels, no 0/1/3).
const nameAlphabet = "bcdfghjklmnpqrstvwxz2456789"

// Suffix returns a 5-character generated-name suffix.
func (g *Generator) Suffix() string {
	v := g.next()
	b := make([]byte, 5)
	for i := range b {
		b[i] = nameAlphabet[v%uint64(len(nameAlphabet))]
		v /= uint64(len(nameAlphabet))
	}
	return string(b)
}

// State is one lab's simulated cluster. It is not safe for concurrent use; the lab's row lock
// serializes access (ADR-0004).
type State struct {
	objects map[Key]Object
	// Now is the logical clock (ADR-0006).
	Now              time.Time
	Gen              Generator
	Revision         int64
	CurrentNamespace string
	Images           ImageCatalog
	Limits           Limits

	events  []Event
	changed map[Key]bool
	deleted map[Key]bool
}

// NewState returns an empty state at the given logical time.
func NewState(now time.Time, seed uint64, images ImageCatalog) *State {
	return &State{
		objects: map[Key]Object{}, Now: now.UTC(), Gen: Generator{Seed: seed},
		CurrentNamespace: "default", Images: images, Limits: DefaultLimits,
		changed: map[Key]bool{}, deleted: map[Key]bool{},
	}
}

// Restore loads previously persisted objects without marking them changed.
func (s *State) Restore(objs []Object) error {
	for _, o := range objs {
		k := KeyOf(o)
		if _, dup := s.objects[k]; dup {
			return fmt.Errorf("duplicate object %s", k)
		}
		s.objects[k] = o
		if rv, err := strconv.ParseInt(o.Meta().ResourceVersion, 10, 64); err == nil && rv > s.Revision {
			s.Revision = rv
		}
	}
	return nil
}

// Get returns a copy of the object, or nil.
func (s *State) Get(kind, namespace, name string) Object {
	if ClusterScoped(kind) {
		namespace = ""
	}
	o, ok := s.objects[Key{kind, namespace, name}]
	if !ok {
		return nil
	}
	return o.DeepCopy()
}

// Has reports whether an object exists.
func (s *State) Has(kind, namespace, name string) bool {
	if ClusterScoped(kind) {
		namespace = ""
	}
	_, ok := s.objects[Key{kind, namespace, name}]
	return ok
}

// List returns copies of objects of kind (all namespaces if namespace is "*"), sorted by
// namespace then name, as kubectl does.
func (s *State) List(kind, namespace string) []Object {
	var out []Object
	for k, o := range s.objects {
		if k.Kind != kind {
			continue
		}
		if namespace != "*" && !ClusterScoped(kind) && k.Namespace != namespace {
			continue
		}
		out = append(out, o.DeepCopy())
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Meta(), out[j].Meta()
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		return a.Name < b.Name
	})
	return out
}

// Count returns the number of objects (Events excluded; they aren't objects here).
func (s *State) Count() int { return len(s.objects) }

// Put creates or replaces an object. New objects get a UID and creation time; every write gets
// the next resource version. Generation increases when a workload's spec changes.
func (s *State) Put(o Object) error {
	k := KeyOf(o)
	if ClusterScoped(k.Kind) {
		o.Meta().Namespace = ""
		k.Namespace = ""
	}
	m := o.Meta()
	old, exists := s.objects[k]
	if !exists {
		if s.Count() >= s.Limits.MaxObjects {
			return fmt.Errorf("%w: at most %d objects per lab", ErrLimit, s.Limits.MaxObjects)
		}
		if m.UID == "" {
			m.UID = s.Gen.UID()
		}
		if m.CreationTimestamp.IsZero() {
			m.CreationTimestamp = s.Now
		}
		if m.Generation == 0 && hasGeneration(k.Kind) {
			m.Generation = 1
		}
	} else {
		om := old.Meta()
		m.UID, m.CreationTimestamp = om.UID, om.CreationTimestamp
		if hasGeneration(k.Kind) {
			m.Generation = om.Generation
			if !jsonEqual(specOf(old), specOf(o)) {
				m.Generation++
			}
		}
		if jsonEqual(old, o) {
			return nil // no-op write: keep the resource version
		}
	}
	s.Revision++
	m.ResourceVersion = strconv.FormatInt(s.Revision, 10)
	TypeMetaFor(o)
	s.objects[k] = o.DeepCopy()
	s.changed[k] = true
	delete(s.deleted, k)
	return nil
}

// Delete removes an object. Dependents are removed by the garbage collector on reconcile.
func (s *State) Delete(kind, namespace, name string) bool {
	if ClusterScoped(kind) {
		namespace = ""
	}
	k := Key{kind, namespace, name}
	if _, ok := s.objects[k]; !ok {
		return false
	}
	delete(s.objects, k)
	delete(s.changed, k)
	s.deleted[k] = true
	return true
}

// Record appends a simulated event at the current logical time.
func (s *State) Record(eventType, reason string, o Object, message string) {
	m := o.Meta()
	s.events = append(s.events, Event{
		Type: eventType, Reason: reason, Kind: o.GetKind(), Namespace: m.Namespace, Name: m.Name,
		Message: message, Time: s.Now,
	})
}

// Events returns events recorded since the state was loaded.
func (s *State) Events() []Event { return s.events }

// Changes returns the objects written and the keys deleted since load, in key order.
func (s *State) Changes() (upserted []Object, deleted []Key) {
	for k := range s.changed {
		upserted = append(upserted, s.objects[k].DeepCopy())
	}
	for k := range s.deleted {
		deleted = append(deleted, k)
	}
	sort.Slice(upserted, func(i, j int) bool { return KeyOf(upserted[i]).String() < KeyOf(upserted[j]).String() })
	sort.Slice(deleted, func(i, j int) bool { return deleted[i].String() < deleted[j].String() })
	return upserted, deleted
}

// All returns copies of every object in stable order (kind, namespace, name).
func (s *State) All() []Object {
	out := make([]Object, 0, len(s.objects))
	for _, o := range s.objects {
		out = append(out, o.DeepCopy())
	}
	sort.Slice(out, func(i, j int) bool { return KeyOf(out[i]).String() < KeyOf(out[j]).String() })
	return out
}

// Tick is how far the logical clock moves for each mutating command (ADR-0006).
const Tick = 10 * time.Second

// Advance moves the logical clock forward (one Tick per mutating command, ADR-0006).
func (s *State) Advance(d time.Duration) { s.Now = s.Now.Add(d) }

func hasGeneration(kind string) bool { return kind == KindDeployment || kind == KindReplicaSet }

func specOf(o Object) any {
	switch v := o.(type) {
	case *Deployment:
		return v.Spec
	case *ReplicaSet:
		return v.Spec
	}
	return nil
}

func jsonEqual(a, b any) bool {
	ja, err1 := json.Marshal(a)
	jb, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && string(ja) == string(jb)
}

// TypeMetaFor fills apiVersion and kind from the Go type.
func TypeMetaFor(o Object) {
	v := reflect.ValueOf(o).Elem()
	tm := v.FieldByName("TypeMeta").Addr().Interface().(*TypeMeta)
	kind := reflect.TypeOf(o).Elem().Name()
	tm.Kind = kind
	tm.APIVersion = APIVersion(kind)
}

// Rebase shifts every resourceVersion up by offset. Reset reloads a lab from its fixture, and
// resource versions must keep increasing across the reset (they are never reused in a lab).
func (s *State) Rebase(offset int64) {
	if offset <= 0 {
		return
	}
	for _, o := range s.objects {
		m := o.Meta()
		if rv, err := strconv.ParseInt(m.ResourceVersion, 10, 64); err == nil {
			m.ResourceVersion = strconv.FormatInt(rv+offset, 10)
		}
	}
	s.Revision += offset
}
