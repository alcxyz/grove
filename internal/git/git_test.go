package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestPushPushesCurrentBranchToUpstream(t *testing.T) {
	root := t.TempDir()
	remote := filepath.Join(root, "remote.git")
	local := filepath.Join(root, "local")

	gitCmd(t, root, "init", "--bare", remote)
	gitCmd(t, root, "init", local)
	gitCmd(t, local, "config", "user.email", "grove@example.test")
	gitCmd(t, local, "config", "user.name", "Grove Test")
	if err := os.WriteFile(filepath.Join(local, "README.md"), []byte("initial\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, local, "add", "README.md")
	gitCmd(t, local, "commit", "-m", "initial")
	gitCmd(t, local, "branch", "-M", "main")
	gitCmd(t, local, "remote", "add", "origin", remote)
	gitCmd(t, local, "push", "-u", "origin", "main")

	if err := os.WriteFile(filepath.Join(local, "README.md"), []byte("updated\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, local, "commit", "-am", "update")
	localHead := gitCmd(t, local, "rev-parse", "HEAD")

	if _, err := Push(local); err != nil {
		t.Fatal(err)
	}
	remoteHead := gitCmd(t, root, "--git-dir", remote, "rev-parse", "main")
	if remoteHead != localHead {
		t.Fatalf("remote head = %s, want %s", remoteHead, localHead)
	}
}
