package grading

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// The assertion DSL (ADR-0007): dotted paths with [n] list indexes and ["key"] map keys, and a
// fixed set of operators. No expressions, wildcards or scripts.

type segment struct {
	key   string
	index int // -1 for a map key
}

// parsePath splits a resource path such as spec.containers[0].image or
// metadata.annotations["kubernetes.io/change-cause"].
func parsePath(path string) ([]segment, error) {
	var out []segment
	i := 0
	field := func() error {
		start := i
		for i < len(path) && path[i] != '.' && path[i] != '[' {
			i++
		}
		if i == start {
			return fmt.Errorf("invalid path %q", path)
		}
		out = append(out, segment{key: path[start:i], index: -1})
		return nil
	}
	if err := field(); err != nil {
		return nil, err
	}
	for i < len(path) {
		switch path[i] {
		case '.':
			i++
			if err := field(); err != nil {
				return nil, err
			}
		case '[':
			end := strings.IndexByte(path[i:], ']')
			if end < 0 {
				return nil, fmt.Errorf("invalid path %q", path)
			}
			inner := path[i+1 : i+end]
			i += end + 1
			if k, ok := strings.CutPrefix(inner, `"`); ok {
				k, ok = strings.CutSuffix(k, `"`)
				if !ok || k == "" {
					return nil, fmt.Errorf("invalid path %q", path)
				}
				out = append(out, segment{key: k, index: -1})
				continue
			}
			n, err := strconv.Atoi(inner)
			if err != nil || n < 0 {
				return nil, fmt.Errorf("invalid path %q", path)
			}
			out = append(out, segment{index: n})
		default:
			return nil, fmt.Errorf("invalid path %q", path)
		}
	}
	return out, nil
}

// Lookup returns the value at path in a JSON document, and whether it exists.
func Lookup(doc any, path string) (any, bool, error) {
	segs, err := parsePath(path)
	if err != nil {
		return nil, false, err
	}
	cur := doc
	for _, s := range segs {
		if s.index >= 0 {
			list, ok := cur.([]any)
			if !ok || s.index >= len(list) {
				return nil, false, nil
			}
			cur = list[s.index]
			continue
		}
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false, nil
		}
		if cur, ok = m[s.key]; !ok {
			return nil, false, nil
		}
	}
	return cur, cur != nil, nil
}

// Check evaluates one assertion against a JSON document.
func Check(doc any, a Assertion) (bool, error) {
	v, found, err := Lookup(doc, a.Path)
	if err != nil {
		return false, err
	}
	switch a.Operator {
	case "exists":
		return found, nil
	case "notExists":
		return !found, nil
	case "equals":
		return found && equal(v, a.Value), nil
	case "notEquals":
		return !found || !equal(v, a.Value), nil
	case "in":
		list, ok := a.Value.([]any)
		if !ok {
			return false, fmt.Errorf("in needs a list")
		}
		for _, want := range list {
			if found && equal(v, want) {
				return true, nil
			}
		}
		return false, nil
	case "contains":
		switch x := v.(type) {
		case []any:
			for _, e := range x {
				if equal(e, a.Value) {
					return true, nil
				}
			}
			return false, nil
		case string:
			s, ok := a.Value.(string)
			return ok && strings.Contains(x, s), nil
		}
		return false, nil
	case "greaterThanOrEqual", "lessThanOrEqual":
		if !found {
			return false, nil
		}
		c, ok := compare(v, a.Value)
		if !ok {
			return false, nil
		}
		if a.Operator == "greaterThanOrEqual" {
			return c >= 0, nil
		}
		return c <= 0, nil
	case "lengthEquals", "lengthGreaterThanOrEqual":
		want, ok := number(a.Value)
		if !ok {
			return false, fmt.Errorf("%s needs a number", a.Operator)
		}
		n := 0
		switch x := v.(type) {
		case []any:
			n = len(x)
		case map[string]any:
			n = len(x)
		case string:
			n = len(x)
		}
		if a.Operator == "lengthEquals" {
			return float64(n) == want, nil
		}
		return float64(n) >= want, nil
	}
	return false, fmt.Errorf("unknown operator %q", a.Operator)
}

// equal compares scalars: numbers by value (so a "3" annotation equals 3), CPU and memory
// quantities by amount ("500m" equals "0.5"), everything else exactly.
func equal(a, b any) bool {
	if c, ok := compare(a, b); ok {
		return c == 0
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}

// compare orders two numbers, or two CPU or memory quantities ("500m" < "1", "512Mi" < "1Gi").
func compare(a, b any) (int, bool) {
	if x, ok := number(a); ok {
		if y, ok := number(b); ok {
			return sign(x - y), true
		}
	}
	as, bs := scalarString(a), scalarString(b)
	if x, y, ok := quantities(as, bs); ok {
		return sign(float64(x - y)), true
	}
	return 0, false
}

// quantities parses both as CPU (when both are plain or millicore numbers) or as memory.
func quantities(a, b string) (int64, int64, bool) {
	cpuLike := func(s string) bool {
		_, err := strconv.ParseFloat(strings.TrimSuffix(s, "m"), 64)
		return err == nil
	}
	if cpuLike(a) && cpuLike(b) {
		x, err1 := simulator.ParseCPU(a)
		y, err2 := simulator.ParseCPU(b)
		return x, y, err1 == nil && err2 == nil
	}
	x, err1 := simulator.ParseMemory(a)
	y, err2 := simulator.ParseMemory(b)
	return x, y, err1 == nil && err2 == nil
}

func sign(f float64) int {
	switch {
	case f < 0:
		return -1
	case f > 0:
		return 1
	}
	return 0
}

// number reads a JSON number, or a string that is a plain decimal number.
func number(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case string:
		f, err := strconv.ParseFloat(x, 64)
		return f, err == nil
	}
	return 0, false
}
