package simulator

import (
	"strings"
)

// ImageProfile says how containers from an image behave (simulator-compatibility.md §2).
type ImageProfile struct {
	Image    string `json:"image"`
	Behavior string `json:"behavior"` // "run" or "crash"
	ExitCode int32  `json:"exitCode,omitempty"`
	// RequiresEnv: for "crash", the container crashes only when one of these env vars is missing.
	RequiresEnv []string `json:"requiresEnv,omitempty"`
	Logs        []string `json:"logs,omitempty"`
}

// ImageCatalog resolves image references. Anything not in the catalog fails to pull.
type ImageCatalog struct {
	byRef map[string]ImageProfile
}

func runImage(ref string, logs ...string) ImageProfile {
	return ImageProfile{Image: ref, Behavior: "run", Logs: logs}
}

var nginxLogs = []string{
	"/docker-entrypoint.sh: Configuration complete; ready for start up",
	"nginx: start worker processes",
}

// builtinImages are available in every lab.
var builtinImages = []ImageProfile{
	runImage("nginx:1.25", nginxLogs...), runImage("nginx:1.26", nginxLogs...), runImage("nginx:1.27", nginxLogs...),
	runImage("nginx:latest", nginxLogs...), runImage("nginx", nginxLogs...),
	runImage("busybox:1.36", "(no output)"), runImage("busybox", "(no output)"),
	runImage("redis:7", "Ready to accept connections tcp"), runImage("redis", "Ready to accept connections tcp"),
	runImage("httpd:2.4", "AH00094: Command line: 'httpd -D FOREGROUND'"),
	runImage("kubedojo/demo-api:1.0", "demo-api 1.0 starting", "listening on 0.0.0.0:8080"),
	runImage("kubedojo/demo-api:2.0", "demo-api 2.0 starting", "listening on 0.0.0.0:8080"),
}

// NewImageCatalog returns the built-in catalog extended (and overridden) by fixture entries.
func NewImageCatalog(extra []ImageProfile) ImageCatalog {
	c := ImageCatalog{byRef: map[string]ImageProfile{}}
	for _, p := range builtinImages {
		c.byRef[p.Image] = p
	}
	for _, p := range extra {
		c.byRef[p.Image] = p
	}
	return c
}

// Lookup returns the profile for an image reference. "docker.io/library/" prefixes are
// normalized like the container runtime does.
func (c ImageCatalog) Lookup(ref string) (ImageProfile, bool) {
	if p, ok := c.byRef[ref]; ok {
		return p, true
	}
	for _, prefix := range []string{"docker.io/library/", "docker.io/"} {
		if strings.HasPrefix(ref, prefix) {
			if p, ok := c.byRef[strings.TrimPrefix(ref, prefix)]; ok {
				return p, true
			}
		}
	}
	return ImageProfile{}, false
}

// Extra returns the non-built-in entries (persisted with the lab's fixture).
func (c ImageCatalog) Extra() []ImageProfile {
	var out []ImageProfile
	builtin := map[string]bool{}
	for _, p := range builtinImages {
		builtin[p.Image] = true
	}
	for _, ref := range sortedKeys(c.byRef) {
		if !builtin[ref] {
			out = append(out, c.byRef[ref])
		}
	}
	return out
}
