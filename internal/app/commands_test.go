package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/alcxyz/grove/internal/config"
)

func TestDiscoverRepoPathsHonorsProfileExcludes(t *testing.T) {
	base := t.TempDir()
	for _, name := range []string{"dms-plugins", "forge-tidy", "grove.bak"} {
		if err := os.MkdirAll(filepath.Join(base, name, ".git"), 0o755); err != nil {
			t.Fatalf("create test repo %s: %v", name, err)
		}
	}

	got := discoverRepoPaths(config.Profile{
		BasePaths:    []string{base},
		Prefixes:     []string{},
		ExcludePaths: []string{filepath.Join(base, "dms-plugins")},
		ExcludeRepos: []string{"grove.bak"},
	})

	want := []string{filepath.Join(base, "forge-tidy")}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("discoverRepoPaths() = %v, want %v", got, want)
	}
}

func TestDiscoverRepoPathsIncludesExactRepoPaths(t *testing.T) {
	base := t.TempDir()
	repoPath := filepath.Join(base, "grove")
	if err := os.MkdirAll(filepath.Join(repoPath, ".git"), 0o755); err != nil {
		t.Fatalf("create exact repo path: %v", err)
	}

	got := discoverRepoPaths(config.Profile{
		BasePaths: []string{filepath.Join(base, "empty")},
		Prefixes:  []string{"does-not-match-"},
		RepoPaths: []string{repoPath},
	})

	if len(got) != 1 || got[0] != repoPath {
		t.Fatalf("discoverRepoPaths() = %v, want [%s]", got, repoPath)
	}
}

func TestEditorCommandSplitsArguments(t *testing.T) {
	t.Setenv("EDITOR", "code --wait")
	bin, args := editorCommand()
	if bin != "code" || len(args) != 1 || args[0] != "--wait" {
		t.Fatalf("editorCommand() = %q %q, want code [--wait]", bin, args)
	}
	t.Setenv("EDITOR", "")
	if bin, args := editorCommand(); bin != "nvim" || len(args) != 0 {
		t.Fatalf("editorCommand() default = %q %q, want nvim", bin, args)
	}
}

func TestEditorCommandHonoursQuotes(t *testing.T) {
	t.Setenv("EDITOR", `code --user-data-dir "/tmp/editor profile" --wait`)
	bin, args := editorCommand()
	want := []string{"--user-data-dir", "/tmp/editor profile", "--wait"}
	if bin != "code" || strings.Join(args, "|") != strings.Join(want, "|") {
		t.Fatalf("editorCommand() = %q %q, want code %q", bin, args, want)
	}
}

func TestSplitWordsWithoutBackslashEscapes(t *testing.T) {
	got := splitWords(`C:\tools\nvim.exe -u "C:\my init.vim"`, false)
	want := []string{`C:\tools\nvim.exe`, "-u", `C:\my init.vim`}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("splitWords = %q, want %q", got, want)
	}
}

func TestEditorCommandUsesExecutablePathWithSpaces(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("test editor has no .exe extension")
	}
	dir := filepath.Join(t.TempDir(), "My Editor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	editor := filepath.Join(dir, "edit")
	if err := os.WriteFile(editor, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", editor)
	if bin, args := editorCommand(); bin != editor || len(args) != 0 {
		t.Fatalf("editorCommand() = %q %q, want %q", bin, args, editor)
	}
}

func TestSplitWords(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{`vim`, []string{"vim"}},
		{`  emacs   -nw `, []string{"emacs", "-nw"}},
		{`a 'b c' "d e"`, []string{"a", "b c", "d e"}},
		{`a\ b`, []string{"a b"}},
		{`"say \"hi\" \n"`, []string{`say "hi" \n`}},
		{`'it''s'`, []string{"its"}},
		{`x ""`, []string{"x", ""}},
		{`$HOME`, []string{"$HOME"}},
	} {
		got := splitWords(tc.in, true)
		if strings.Join(got, "|") != strings.Join(tc.want, "|") || len(got) != len(tc.want) {
			t.Errorf("splitWords(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
