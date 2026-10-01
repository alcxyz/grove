package cache

import (
	"errors"
	"os"
	"path/filepath"
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
