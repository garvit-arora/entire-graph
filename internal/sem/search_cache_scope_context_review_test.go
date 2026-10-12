package sem

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestCheckoutScopeLookupHonorsCallerDeadline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX Git stub")
	}
	repo := t.TempDir()
	if canonical, err := filepath.EvalSymlinks(repo); err == nil {
		repo = canonical
	}
	if output, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("init: %v %s", err, output)
	}
	bin := t.TempDir()
	if canonical, err := filepath.EvalSymlinks(bin); err == nil {
		bin = canonical
	}
	marker := filepath.Join(bin, "called")
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte("#!/bin/sh\nprintf reached > \"$GRAPH_SCOPE_MARKER\"\nexec sleep 2\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GRAPH_SCOPE_MARKER", marker)
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	scope := cacheCheckoutScope(repo, ctx)
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("Git prefix lookup was not exercised")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("caller deadline ignored: %v", elapsed)
	}
	if scope != "path:"+filepath.Clean(repo) {
		t.Fatalf("failed lookup must fall back to private scope, got %q", scope)
	}
}
