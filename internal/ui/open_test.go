package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
