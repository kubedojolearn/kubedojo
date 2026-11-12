//go:build integration

package labs_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/kubedojolearn/kubedojo/apps/api/internal/labs"
	"github.com/kubedojolearn/kubedojo/apps/api/internal/platform/testdb"
)

const scaledNginx = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  labels: { app: nginx }
spec:
  replicas: 3
  selector:
    matchLabels: { app: nginx }
  template:
    metadata:
      labels: { app: nginx }
    spec:
      containers:
        - name: nginx
          image: nginx:1.27
          ports:
            - containerPort: 80
          resources:
            requests: { cpu: 100m, memory: 64Mi }
`

func filesSetup(t *testing.T) (*labs.Service, uuid.UUID, labs.Lab) {
	t.Helper()
	pool := testdb.New(t)
	svc := labs.NewService(pool)
	user := testdb.CreateUser(t, pool, "learner")
	challenge, _ := json.Marshal(map[string]any{
		"version":      2,
		"initialFiles": []map[string]string{{"path": "manifests/nginx.yaml", "content": strings.Replace(scaledNginx, "replicas: 3", "replicas: 1", 1)}},
	})
	lesson := testdb.CreateLaunchableChallenge(t, pool, "files", string(challenge), nginxFixture(t))
	lab, _, err := svc.Create(ctx, user, lesson.VersionID)
	if err != nil {
		t.Fatal(err)
	}
	return svc, user, lab
}

func TestLabsStartWithTheLessonsFiles(t *testing.T) {
	svc, user, lab := filesSetup(t)
	files, err := svc.Files(ctx, user, lab.ID)
	if err != nil || len(files) != 1 || files[0].Path != "manifests/nginx.yaml" || !strings.Contains(files[0].Content, "replicas: 1") {
		t.Fatalf("files: %v %+v", err, files)
	}
	if files[0].SizeBytes != len(files[0].Content) {
		t.Fatalf("size %d", files[0].SizeBytes)
	}
}

func TestSavedFilesFeedKubectlApply(t *testing.T) {
	svc, user, lab := filesSetup(t)
	files, _ := svc.Files(ctx, user, lab.ID)
	saved, err := svc.SaveFile(ctx, user, lab.ID, "manifests/nginx.yaml", scaledNginx, &files[0].UpdatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if l, _ := svc.Get(ctx, user, lab.ID); l.Version != 1 {
		t.Fatalf("saving a file changed the lab version to %d", l.Version)
	}
	res, err := svc.RunCommand(ctx, user, lab.ID, uuid.New(), "kubectl apply -f manifests/nginx.yaml")
	if err != nil || res.ExitCode != 0 || !res.Mutated || !strings.Contains(res.Stdout, "deployment.apps/nginx configured") {
		t.Fatalf("apply: %v %+v", err, res)
	}
	pods, _ := svc.RunCommand(ctx, user, lab.ID, uuid.New(), "kubectl get pods --no-headers")
	if n := strings.Count(pods.Stdout, "Running"); n != 3 {
		t.Fatalf("want 3 Pods, got %d", n)
	}

	// A save based on an old copy is refused; one based on the current copy works.
	if _, err := svc.SaveFile(ctx, user, lab.ID, "manifests/nginx.yaml", "x: 1\n", &files[0].UpdatedAt); !errors.Is(err, labs.ErrStaleFile) {
		t.Fatalf("stale save: %v", err)
	}
	if _, err := svc.SaveFile(ctx, user, lab.ID, "manifests/nginx.yaml", "x: 1\n", &saved.UpdatedAt); err != nil {
		t.Fatalf("save on the latest copy: %v", err)
	}
	// A new file with a base time didn't exist when the client loaded it: stale too.
	if _, err := svc.SaveFile(ctx, user, lab.ID, "manifests/new.yaml", "x: 1\n", &saved.UpdatedAt); !errors.Is(err, labs.ErrStaleFile) {
		t.Fatalf("new file with base: %v", err)
	}

	// Reset restores the lesson's files as well as its cluster.
	if _, err := svc.Reset(ctx, user, lab.ID, "reset-files-01"); err != nil {
		t.Fatal(err)
	}
	after, _ := svc.Files(ctx, user, lab.ID)
	if len(after) != 1 || !strings.Contains(after[0].Content, "replicas: 1") {
		t.Fatalf("after reset: %+v", after)
	}
}

func TestFileLimitsAndOwnership(t *testing.T) {
	svc, user, lab := filesSetup(t)
	for _, bad := range []struct{ path, content string }{
		{"../etc/passwd", "x"}, {"manifests/Web.yaml", "x"}, {"manifests/web.json", "x"},
		{"manifests/a.yaml", strings.Repeat("a", labs.MaxFileBytes+1)}, {"manifests/a.yaml", "a\x00b"}, {"manifests/a.yaml", "\xff"},
	} {
		if _, err := svc.SaveFile(ctx, user, lab.ID, bad.path, bad.content, nil); !errors.Is(err, labs.ErrInvalidFile) {
			t.Errorf("%q: %v", bad.path, err)
		}
	}
	for i := 1; i < labs.MaxFiles; i++ { // the lesson already gave one
		if _, err := svc.SaveFile(ctx, user, lab.ID, fmt.Sprintf("manifests/f%d.yaml", i), "a: 1\n", nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.SaveFile(ctx, user, lab.ID, "manifests/one-more.yaml", "a: 1\n", nil); !errors.Is(err, labs.ErrFileLimit) {
		t.Fatalf("limit: %v", err)
	}
	if err := svc.DeleteFile(ctx, user, lab.ID, "manifests/f1.yaml"); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteFile(ctx, user, lab.ID, "manifests/f1.yaml"); !errors.Is(err, labs.ErrFileNotFound) {
		t.Fatalf("second delete: %v", err)
	}
	if _, err := svc.SaveFile(ctx, user, lab.ID, "manifests/one-more.yaml", "a: 1\n", nil); err != nil {
		t.Fatalf("after delete: %v", err)
	}

	other := uuid.New()
	if _, err := svc.Files(ctx, other, lab.ID); !errors.Is(err, labs.ErrNotFound) {
		t.Fatalf("other user list: %v", err)
	}
	if _, err := svc.SaveFile(ctx, other, lab.ID, "manifests/x.yaml", "a: 1\n", nil); !errors.Is(err, labs.ErrNotFound) {
		t.Fatalf("other user save: %v", err)
	}
	if _, err := svc.Finish(ctx, user, lab.ID, "finish-files-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SaveFile(ctx, user, lab.ID, "manifests/x.yaml", "a: 1\n", nil); !errors.Is(err, labs.ErrNotActive) {
		t.Fatalf("ended lab: %v", err)
	}
}
