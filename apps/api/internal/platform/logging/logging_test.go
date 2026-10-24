package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewRedactsSensitiveKeys(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, "debug")
	log.Info("request",
		"authorization", "Bearer abc.def.ghi",
		"ws_ticket", "t0p",
		"path", "/api/v1/healthz",
	)
	log.WithGroup("db").Info("connect", "database_url", "postgres://u:pw@h/db")

	out := buf.String()
	for _, leaked := range []string{"abc.def.ghi", "t0p", "pw@h"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("log output leaks %q: %s", leaked, out)
		}
	}
	first := map[string]any{}
	if err := json.Unmarshal([]byte(strings.SplitN(out, "\n", 2)[0]), &first); err != nil {
		t.Fatalf("not JSON: %v", err)
	}
	if first["path"] != "/api/v1/healthz" || first["authorization"] != Redacted {
		t.Fatalf("unexpected record: %v", first)
	}
}

func TestNewFallsBackToInfo(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, "nonsense")
	log.Debug("hidden")
	log.Info("shown")
	if strings.Contains(buf.String(), "hidden") || !strings.Contains(buf.String(), "shown") {
		t.Fatalf("unexpected output: %s", buf.String())
	}
}
