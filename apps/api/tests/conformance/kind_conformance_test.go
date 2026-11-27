//go:build conformance

// Package conformance replays KubeDojo lessons against a real Kubernetes cluster (kind in CI,
// ADR-0002) and compares the resulting object shapes against the in-process simulator to catch
// Kubernetes API/behavioral drift.
package conformance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"testing"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/content"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/kubectl"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/simulator"
)

type challengeSpec struct {
	Challenge struct {
		ID           string `json:"id"`
		InitialFiles []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"initialFiles"`
		Solution struct {
			Files []struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			} `json:"files"`
			Commands []string `json:"commands"`
		} `json:"solution"`
	} `json:"challenge"`
	Fixture simulator.Fixture `json:"fixture"`
}

// TestSimulatorMatchesKindObjectShapes validates that objects created by KubeDojo's fixtures and
// solution manifests are accepted by a real Kubernetes API server (server-side dry-run) with the
// same apiVersion, kind, labels and core spec shape as the simulator.
func TestSimulatorMatchesKindObjectShapes(t *testing.T) {
	if _, err := exec.LookPath("kubectl"); err != nil {
		t.Fatalf("kubectl not found in PATH: %v", err)
	}
	if out, err := exec.Command("kubectl", "cluster-info").CombinedOutput(); err != nil {
		t.Fatalf("kubectl cluster-info failed (start a kind cluster first): %v\n%s", err, out)
	}

	bundle, err := content.Load("../../../../content")
	if err != nil {
		t.Fatalf("load content: %v", err)
	}
	ids := make([]string, 0, len(bundle.Challenges))
	for id := range bundle.Challenges {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			var s challengeSpec
			if err := json.Unmarshal(bundle.Challenges[id].Spec, &s); err != nil {
				t.Fatalf("unmarshal spec: %v", err)
			}
			st, err := simulator.Load(s.Fixture, 42)
			if err != nil {
				t.Fatalf("load fixture: %v", err)
			}
			files := map[string]string{}
			for _, f := range s.Challenge.InitialFiles {
				files[f.Path] = f.Content
			}
			for _, f := range s.Challenge.Solution.Files {
				files[f.Path] = f.Content
			}
			env := kubectl.Env{State: st, Events: append([]simulator.Event{}, st.Events()...), Files: files}
			for _, cmd := range s.Challenge.Solution.Commands {
				res := kubectl.Execute(cmd, env)
				if res.Err != nil {
					t.Fatalf("simulator error on %q: %v", cmd, res.Err)
				}
			}

			// Dry-run every namespaced workload/config/service object from the final simulator
			// state against the real kind API server to confirm schema & apiVersion conformance.
			ns := "conformance-" + strings.NewReplacer("/", "-", "_", "-").Replace(id)
			if len(ns) > 63 {
				ns = ns[:63]
			}
			if out, err := exec.Command("kubectl", "create", "namespace", ns, "--dry-run=server", "-o", "json").CombinedOutput(); err != nil {
				t.Fatalf("namespace dry-run failed: %v\n%s", err, out)
			}

			for _, obj := range st.All() {
				kind := obj.GetKind()
				if !isDryRunConformantKind(kind) {
					continue
				}
				rawObj, err := simulator.MarshalForOutput(obj)
				if err != nil {
					t.Fatalf("marshal %s/%s: %v", kind, obj.Meta().Name, err)
				}
				var doc map[string]any
				if err := json.Unmarshal(rawObj, &doc); err != nil {
					t.Fatalf("unmarshal %s/%s: %v", kind, obj.Meta().Name, err)
				}
				delete(doc, "status")
				if md, ok := doc["metadata"].(map[string]any); ok {
					md["namespace"] = "default"
					delete(md, "uid")
					delete(md, "resourceVersion")
					delete(md, "creationTimestamp")
					delete(md, "generation")
					delete(md, "ownerReferences")
				}
				// Strip simulated-only pod fields that the API server rejects on creation.
				if kind == simulator.KindPod {
					if sp, ok := doc["spec"].(map[string]any); ok {
						delete(sp, "nodeName")
					}
				}
				// Strip allocated ClusterIP on Services so dry-run doesn't conflict with kind's CIDR.
				if kind == simulator.KindService {
					if sp, ok := doc["spec"].(map[string]any); ok {
						delete(sp, "clusterIP")
					}
				}
				raw, err := json.Marshal(doc)
				if err != nil {
					t.Fatalf("marshal %s/%s: %v", kind, obj.Meta().Name, err)
				}
				cmd := exec.Command("kubectl", "apply", "--dry-run=server", "--validate=strict", "-f", "-", "-o", "json")
				cmd.Stdin = bytes.NewReader(raw)
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Errorf("%s/%s rejected by kind API server: %v\n%s", kind, obj.Meta().Name, err, out)
					continue
				}
				var realObj struct {
					APIVersion string `json:"apiVersion"`
					Kind       string `json:"kind"`
					Metadata   struct {
						Name   string            `json:"name"`
						Labels map[string]string `json:"labels"`
					} `json:"metadata"`
				}
				if err := json.Unmarshal(out, &realObj); err != nil {
					t.Errorf("%s/%s unmarshal kind response: %v", kind, obj.Meta().Name, err)
					continue
				}
				simAPIVersion := simulator.APIVersion(kind)
				if realObj.APIVersion != simAPIVersion || realObj.Kind != kind {
					t.Errorf("%s/%s shape drift: simulator=(%s, %s) kind=(%s, %s)",
						kind, obj.Meta().Name, simAPIVersion, kind, realObj.APIVersion, realObj.Kind)
				}
				for lk, lv := range obj.Meta().Labels {
					if realObj.Metadata.Labels[lk] != lv {
						t.Errorf("%s/%s label %q drift: sim=%q kind=%q", kind, obj.Meta().Name, lk, lv, realObj.Metadata.Labels[lk])
					}
				}
			}
		})
	}
}

func isDryRunConformantKind(kind string) bool {
	switch kind {
	case "Pod", "Deployment", "Service", "ConfigMap", "Secret":
		return true
	default:
		return false
	}
}

// Keep fmt imported if error formatting evolves.
var _ = fmt.Sprintf
