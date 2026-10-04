package ui

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func fakeLauncher(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	name := "xdg-open"
	if runtime.GOOS == "darwin" {
		name = "open"
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	return dir
}

func TestOpenURLPassesURLToLauncher(t *testing.T) {
	record := filepath.Join(t.TempDir(), "url")
	fakeLauncher(t, `printf '%s' "$1" > "`+record+`"`+"\n")
	if err := OpenURL("https://example.test/pr/1"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "https://example.test/pr/1" {
		t.Fatalf("launcher got %q", got)
	}
}

func TestOpenURLReportsLauncherFailure(t *testing.T) {
	fakeLauncher(t, "exit 3\n")
	err := OpenURL("https://example.test")
	if err == nil || !strings.Contains(err.Error(), "exit status 3") {
		t.Fatalf("err = %v, want launcher exit status", err)
	}
}

func TestOpenURLRejectsEmptyURL(t *testing.T) {
	if err := OpenURL(""); err == nil {
		t.Fatal("expected error for empty URL")
	}
}

func TestOpenURLStopsWaitingForSlowLauncher(t *testing.T) {
	old := launcherWait
	launcherWait = 100 * time.Millisecond
	t.Cleanup(func() { launcherWait = old })
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep not available")
	}
	fakeLauncher(t, "exec "+sleep+" 10\n")
	start := time.Now()
	if err := OpenURL("https://example.test"); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("OpenURL waited %s for a launcher that does not exit", elapsed)
	}
}
