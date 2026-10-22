// Package buildinfo exposes version metadata injected at link time:
//
//	go build -ldflags "-X github.com/kubedojolearn/kubedojo/apps/api/internal/platform/buildinfo.Version=0.1.0 \
//	                   -X github.com/kubedojolearn/kubedojo/apps/api/internal/platform/buildinfo.Commit=abc1234"
package buildinfo

var (
	// Version is the release version, or "dev" for local builds.
	Version = "dev"
	// Commit is the short Git commit the binary was built from.
	Commit = "unknown"
)
