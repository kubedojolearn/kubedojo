package simulator

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Selector matches label sets. It is built from a LabelSelector or parsed from kubectl's
// -l syntax: equality (a=b, a==b, a!=b) and set-based (a in (x,y), a notin (x), a, !a).
type Selector struct {
	reqs []requirement
}

type requirement struct {
	key    string
	op     string // "=", "!=", "in", "notin", "exists", "!"
	values []string
}

// Empty reports whether the selector matches everything.
func (s Selector) Empty() bool { return len(s.reqs) == 0 }

// Matches reports whether labels satisfy every requirement.
func (s Selector) Matches(labels map[string]string) bool {
	for _, r := range s.reqs {
		v, ok := labels[r.key]
		switch r.op {
		case "=":
			if !ok || v != r.values[0] {
				return false
			}
		case "!=":
			if ok && v == r.values[0] {
				return false
			}
		case "in":
			if !ok || !contains(r.values, v) {
				return false
			}
		case "notin":
			if ok && contains(r.values, v) {
				return false
			}
		case "exists":
			if !ok {
				return false
			}
		case "!":
			if ok {
				return false
			}
		}
	}
	return true
}

// String renders the selector in kubectl syntax (used by describe).
func (s Selector) String() string {
	parts := make([]string, 0, len(s.reqs))
	for _, r := range s.reqs {
		switch r.op {
		case "=", "!=":
			parts = append(parts, r.key+r.op+r.values[0])
		case "in", "notin":
			parts = append(parts, fmt.Sprintf("%s %s (%s)", r.key, r.op, strings.Join(r.values, ",")))
		case "exists":
			parts = append(parts, r.key)
		case "!":
			parts = append(parts, "!"+r.key)
		}
	}
	return strings.Join(parts, ",")
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

// SelectorFromLabels matches exactly the given labels (Service selectors).
func SelectorFromLabels(labels map[string]string) Selector {
	keys := sortedKeys(labels)
	s := Selector{}
	for _, k := range keys {
		s.reqs = append(s.reqs, requirement{key: k, op: "=", values: []string{labels[k]}})
	}
	return s
}

// SelectorFromLabelSelector converts a metav1.LabelSelector.
func SelectorFromLabelSelector(ls LabelSelector) (Selector, error) {
	s := SelectorFromLabels(ls.MatchLabels)
	for _, e := range ls.MatchExpressions {
		switch e.Operator {
		case "In", "NotIn":
			if len(e.Values) == 0 {
				return Selector{}, fmt.Errorf("matchExpressions %s %s: values must not be empty", e.Key, e.Operator)
			}
			op := "in"
			if e.Operator == "NotIn" {
				op = "notin"
			}
			s.reqs = append(s.reqs, requirement{key: e.Key, op: op, values: append([]string(nil), e.Values...)})
		case "Exists", "DoesNotExist":
			if len(e.Values) != 0 {
				return Selector{}, fmt.Errorf("matchExpressions %s %s: values must be empty", e.Key, e.Operator)
			}
			op := "exists"
			if e.Operator == "DoesNotExist" {
				op = "!"
			}
			s.reqs = append(s.reqs, requirement{key: e.Key, op: op})
		default:
			return Selector{}, fmt.Errorf("matchExpressions: unsupported operator %q", e.Operator)
		}
	}
	return s, nil
}

var (
	setExpr  = regexp.MustCompile(`^([A-Za-z0-9./_-]+)\s+(in|notin)\s+\(([^()]*)\)$`)
	selValue = regexp.MustCompile(`^(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?$`)
)

// ParseSelector parses kubectl -l syntax.
func ParseSelector(input string) (Selector, error) {
	s := Selector{}
	input = strings.TrimSpace(input)
	if input == "" {
		return s, nil
	}
	for _, term := range splitTerms(input) {
		term = strings.TrimSpace(term)
		if term == "" {
			return Selector{}, fmt.Errorf("found empty term in selector %q", input)
		}
		if m := setExpr.FindStringSubmatch(term); m != nil {
			var values []string
			for _, v := range strings.Split(m[3], ",") {
				v = strings.TrimSpace(v)
				if !selValue.MatchString(v) {
					return Selector{}, fmt.Errorf("invalid label value %q", v)
				}
				values = append(values, v)
			}
			if err := ValidateLabelKey(m[1]); err != nil {
				return Selector{}, err
			}
			s.reqs = append(s.reqs, requirement{key: m[1], op: m[2], values: values})
			continue
		}
		var op string
		var key, value string
		switch {
		case strings.Contains(term, "!="):
			op = "!="
			key, value, _ = strings.Cut(term, "!=")
		case strings.Contains(term, "=="):
			op = "="
			key, value, _ = strings.Cut(term, "==")
		case strings.Contains(term, "="):
			op = "="
			key, value, _ = strings.Cut(term, "=")
		case strings.HasPrefix(term, "!"):
			op, key = "!", strings.TrimPrefix(term, "!")
		default:
			op, key = "exists", term
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if err := ValidateLabelKey(key); err != nil {
			return Selector{}, err
		}
		if op == "=" || op == "!=" {
			if !selValue.MatchString(value) || len(value) > 63 {
				return Selector{}, fmt.Errorf("invalid label value %q", value)
			}
			s.reqs = append(s.reqs, requirement{key: key, op: op, values: []string{value}})
		} else {
			s.reqs = append(s.reqs, requirement{key: key, op: op})
		}
	}
	return s, nil
}

// splitTerms splits on commas that are not inside parentheses.
func splitTerms(s string) []string {
	var out []string
	depth, start := 0, 0
	for i, r := range s {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	return append(out, s[start:])
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
