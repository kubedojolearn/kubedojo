// Package content loads lesson content from the repository's content/ directory and imports it
// into PostgreSQL as immutable challenge versions (ADR-0005). The JSON Schema validator in
// packages/content-schema is the gate in CI; this package re-checks what the import relies on.
package content

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

// Course is content/courses/<slug>.yaml.
type Course struct {
	SchemaVersion int      `json:"schemaVersion"`
	Slug          string   `json:"slug"`
	Title         string   `json:"title"`
	Summary       string   `json:"summary"`
	Description   string   `json:"description"`
	Difficulty    string   `json:"difficulty"`
	DisplayOrder  int32    `json:"displayOrder"`
	Prerequisites []string `json:"prerequisites"`
	Lessons       []string `json:"lessons"`
}

// Challenge is one lesson's content plus its resolved fixture, as stored in challenge_versions.
type Challenge struct {
	ID               string
	Version          int32
	Course           string
	Lesson           string
	Title            string
	Summary          string
	EstimatedMinutes int32
	SimulatorVersion string
	// Spec is the stored document: {"challenge": <challenge>, "fixture": <fixture>}.
	Spec   json.RawMessage
	SHA256 string
}

// Bundle is everything in a content directory.
type Bundle struct {
	Courses    []Course
	Challenges map[string]Challenge // by "<course>/<lesson>"
}

type challengeHeader struct {
	ID               string `json:"id"`
	Version          int32  `json:"version"`
	Course           string `json:"course"`
	Lesson           string `json:"lesson"`
	Title            string `json:"title"`
	Summary          string `json:"summary"`
	EstimatedMinutes int32  `json:"estimatedMinutes"`
	SimulatorVersion string `json:"simulatorVersion"`
	InitialFixture   string `json:"initialFixture"`
}

// Load reads and cross-checks a content directory.
func Load(dir string) (Bundle, error) {
	b := Bundle{Challenges: map[string]Challenge{}}

	courseFiles, err := filepath.Glob(filepath.Join(dir, "courses", "*.yaml"))
	if err != nil {
		return b, err
	}
	sort.Strings(courseFiles)
	for _, f := range courseFiles {
		raw, err := readYAMLAsJSON(f)
		if err != nil {
			return b, err
		}
		var c Course
		if err := strictJSON(raw, &c); err != nil {
			return b, fmt.Errorf("%s: %w", f, err)
		}
		if c.Slug+".yaml" != filepath.Base(f) {
			return b, fmt.Errorf("%s: file name must match slug %q", f, c.Slug)
		}
		b.Courses = append(b.Courses, c)
	}

	challengeFiles, err := filepath.Glob(filepath.Join(dir, "challenges", "*", "*.yaml"))
	if err != nil {
		return b, err
	}
	sort.Strings(challengeFiles)
	for _, f := range challengeFiles {
		raw, err := readYAMLAsJSON(f)
		if err != nil {
			return b, err
		}
		var h challengeHeader
		if err := json.Unmarshal(raw, &h); err != nil {
			return b, fmt.Errorf("%s: %w", f, err)
		}
		if h.ID != h.Course+"/"+h.Lesson || filepath.Base(f) != h.Lesson+".yaml" || filepath.Base(filepath.Dir(f)) != h.Course {
			return b, fmt.Errorf("%s: id, course, lesson and file path must agree", f)
		}
		if h.Version < 1 || h.Title == "" || h.EstimatedMinutes < 1 || h.SimulatorVersion == "" {
			return b, fmt.Errorf("%s: missing version, title, estimatedMinutes or simulatorVersion (run the content validator)", f)
		}
		if !strings.HasPrefix(h.InitialFixture, "fixtures/") || strings.Contains(h.InitialFixture, "..") {
			return b, fmt.Errorf("%s: invalid initialFixture", f)
		}
		fixture, err := readYAMLAsJSON(filepath.Join(dir, h.InitialFixture))
		if err != nil {
			return b, fmt.Errorf("%s: fixture: %w", f, err)
		}
		spec, err := json.Marshal(map[string]json.RawMessage{"challenge": raw, "fixture": fixture})
		if err != nil {
			return b, err
		}
		// json.Marshal sorts map keys, so the encoding (and hash) is canonical for equal content.
		canonical, err := canonicalJSON(spec)
		if err != nil {
			return b, err
		}
		sum := sha256.Sum256(canonical)
		b.Challenges[h.ID] = Challenge{
			ID: h.ID, Version: h.Version, Course: h.Course, Lesson: h.Lesson, Title: h.Title, Summary: h.Summary,
			EstimatedMinutes: h.EstimatedMinutes, SimulatorVersion: h.SimulatorVersion,
			Spec: canonical, SHA256: hex.EncodeToString(sum[:]),
		}
	}

	courseLessons := map[string]bool{}
	for _, c := range b.Courses {
		for _, l := range c.Lessons {
			courseLessons[c.Slug+"/"+l] = true
		}
	}
	for id := range b.Challenges {
		if !courseLessons[id] {
			return b, fmt.Errorf("challenge %s is not listed by its course", id)
		}
	}
	return b, nil
}

func readYAMLAsJSON(path string) (json.RawMessage, error) {
	f, err := os.Open(path) //nolint:gosec // operator-supplied content directory
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var v any
	dec := yaml.NewDecoder(f)
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	out, err := json.Marshal(normalize(v))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return out, nil
}

// normalize turns YAML-decoded values into JSON-compatible ones (dates become YYYY-MM-DD).
func normalize(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			t[k] = normalize(val)
		}
		return t
	case []any:
		for i, val := range t {
			t[i] = normalize(val)
		}
		return t
	case time.Time:
		return t.Format("2006-01-02")
	default:
		return v
	}
}

func canonicalJSON(raw []byte) ([]byte, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

func strictJSON(raw []byte, dst any) error {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return errors.New("unexpected structure (run the content validator): " + err.Error())
	}
	return nil
}
