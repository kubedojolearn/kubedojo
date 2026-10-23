package kubectl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

// humanDuration formats an age like kubectl (k8s.io/apimachinery/pkg/util/duration.HumanDuration).
func humanDuration(d time.Duration) string {
	if seconds := int(d.Seconds()); seconds < -1 {
		return "<invalid>"
	} else if seconds < 0 {
		return "0s"
	} else if seconds < 60*2 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := int(d / time.Minute)
	if minutes < 10 {
		s := int(d/time.Second) % 60
		if s == 0 {
			return fmt.Sprintf("%dm", minutes)
		}
		return fmt.Sprintf("%dm%ds", minutes, s)
	} else if minutes < 60*3 {
		return fmt.Sprintf("%dm", minutes)
	}
	hours := int(d / time.Hour)
	switch {
	case hours < 8:
		m := int(d/time.Minute) % 60
		if m == 0 {
			return fmt.Sprintf("%dh", hours)
		}
		return fmt.Sprintf("%dh%dm", hours, m)
	case hours < 48:
		return fmt.Sprintf("%dh", hours)
	case hours < 24*8:
		h := hours % 24
		if h == 0 {
			return fmt.Sprintf("%dd", hours/24)
		}
		return fmt.Sprintf("%dd%dh", hours/24, h)
	case hours < 24*365*2:
		return fmt.Sprintf("%dd", hours/24)
	case hours < 24*365*8:
		dy := (hours / 24) % 365
		if dy == 0 {
			return fmt.Sprintf("%dy", hours/24/365)
		}
		return fmt.Sprintf("%dy%dd", hours/24/365, dy)
	}
	return fmt.Sprintf("%dy", hours/24/365)
}

func age(now, t time.Time) string {
	if t.IsZero() {
		return "<unknown>"
	}
	return humanDuration(now.Sub(t))
}

// table renders rows like kubectl's printers (tabwriter: min width 6, tab width 4, padding 3).
type table struct {
	header []string
	rows   [][]string
	pad    int // cell padding; kubectl's resource printers use 3
}

func (t *table) add(cells ...string) { t.rows = append(t.rows, cells) }

func (t *table) render(noHeaders bool) string {
	var buf bytes.Buffer
	pad := t.pad
	if pad == 0 {
		pad = 3
	}
	w := tabwriter.NewWriter(&buf, 6, 4, pad, ' ', 0)
	if !noHeaders {
		_, _ = fmt.Fprintln(w, strings.Join(t.header, "\t"))
	}
	for _, r := range t.rows {
		_, _ = fmt.Fprintln(w, strings.Join(r, "\t"))
	}
	_ = w.Flush()
	// tabwriter pads the last column too; kubectl's output has no trailing spaces.
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n") + "\n"
}

func labelString(labels map[string]string) string {
	if len(labels) == 0 {
		return "<none>"
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + labels[k]
	}
	return strings.Join(parts, ",")
}

func orNone(s string) string {
	if s == "" {
		return "<none>"
	}
	return s
}

// toGeneric turns an object (with the simulated annotation) into plain maps for printing.
func toGeneric(o simulator.Object) (any, error) {
	raw, err := simulator.MarshalForOutput(o)
	if err != nil {
		return nil, err
	}
	var v any
	return v, json.Unmarshal(raw, &v)
}

// listOf wraps items the way kubectl prints several objects in -o yaml/json.
func listOf(items []any) any {
	return map[string]any{
		"apiVersion": "v1",
		"kind":       "List",
		"items":      items,
		"metadata":   map[string]any{"resourceVersion": ""},
	}
}

// marshalJSON prints like kubectl -o json: 4-space indent, keys sorted, trailing newline.
func marshalJSON(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "    ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// marshalYAML prints like kubectl -o yaml: keys sorted, 2-space indent, sequences not indented
// under their key, timestamps and ambiguous strings quoted.
func marshalYAML(v any) (string, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	enc.CompactSeqIndent()
	if err := enc.Encode(normalizeNumbers(v)); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// normalizeNumbers turns whole float64s (from JSON decoding) into ints so YAML prints 80, not 80.0.
func normalizeNumbers(v any) any {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			x[k] = normalizeNumbers(e)
		}
	case []any:
		for i, e := range x {
			x[i] = normalizeNumbers(e)
		}
	case float64:
		if x == float64(int64(x)) {
			return int64(x)
		}
	}
	return v
}
