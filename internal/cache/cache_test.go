package cache

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/alcxyz/grove/internal/model"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	prs := []model.PR{{Repo: "org/repo", Number: 7, Title: "Fix"}}
	if err := SavePRs(dir, "key", prs); err != nil {
		t.Fatal(err)
	}
	got, at, err := LoadPRs(dir, "key")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Number != 7 || at.IsZero() {
		t.Fatalf("LoadPRs = %+v at %v", got, at)
	}
	if _, _, err := LoadPRs(dir, "other-key"); !errors.Is(err, ErrConfigChanged) {
		t.Fatalf("err = %v, want ErrConfigChanged", err)
	}
}

func TestSaveCapsEntries(t *testing.T) {
	dir := t.TempDir()
	if err := SaveActivity(dir, "key", make([]model.Commit, maxActivity+5)); err != nil {
		t.Fatal(err)
	}
	got, _, err := LoadActivity(dir, "key")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != maxActivity {
		t.Fatalf("len = %d, want %d", len(got), maxActivity)
	}
}

func TestConcurrentSavesLeaveValidFileAndNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			issues := make([]model.Issue, n*10)
			if err := SaveIssues(dir, "key", issues); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if _, _, err := LoadIssues(dir, "key"); err != nil {
		t.Fatalf("cache file corrupted by concurrent saves: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("leftover temp file %s", e.Name())
		}
	}
}

func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := SaveState(dir, UIState{ActiveProfile: -1}); err != nil {
		t.Fatal(err)
	}
	st, err := LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.ActiveProfile != -1 {
		t.Fatalf("ActiveProfile = %d, want -1", st.ActiveProfile)
	}
}

func TestWriteFileAtomicModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no Unix permission bits")
	}
	dir := t.TempDir()
	fresh := filepath.Join(dir, "fresh.json")
	if err := writeFileAtomic(fresh, []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(fresh); err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("new file mode = %v, %v; want 0600", info.Mode().Perm(), err)
	}

	existing := filepath.Join(dir, "existing.json")
	if err := os.WriteFile(existing, []byte("{}"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(existing, 0o640); err != nil {
		t.Fatal(err)
	}
	if err := writeFileAtomic(existing, []byte("[]")); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(existing); err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("existing file mode = %v, %v; want 0640 kept", info.Mode().Perm(), err)
	}
}

func TestWriteFileAtomicFollowsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks unavailable:", err)
	}
	if err := writeFileAtomic(link, []byte("[]")); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("link was replaced: %v, %v", info, err)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != "[]" {
		t.Fatalf("target = %q, %v; want []", got, err)
	}
}
