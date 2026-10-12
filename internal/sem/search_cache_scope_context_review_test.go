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

func TestRecordsCacheLoadHonorsCallerDeadline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX Git stub")
	}
	first, second := scopeWorktrees(t)
	cacheDir := t.TempDir()
	header := storeScopeRecords(t, first, cacheDir)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	replay, err := BeginProviderRecordsCache(ctx, second, "test-v1", header.Commit, header.Tree, "snapshot", cacheDir, ProviderSnapshotOptions{Profile: ProfileFull})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, hit := replay.Load(); !hit {
		t.Fatal("control: cross-checkout entry must be a hit")
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
	timer := time.AfterFunc(100*time.Millisecond, cancel)
	defer timer.Stop()
	started := time.Now()
	if _, _, hit := replay.Load(); hit {
		t.Fatal("canceled scope lookup must not share the entry")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("cross-checkout Git lookup was not exercised")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("record cache load ignored caller cancellation: %v", elapsed)
	}
}
