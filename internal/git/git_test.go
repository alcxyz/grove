package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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

func TestRunErrorsIncludeGitStderr(t *testing.T) {
	dir := t.TempDir()
	gitCmd(t, dir, "init")
	_, err := run(dir, "rev-parse", "--verify", "does-not-exist^{commit}")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "fatal:") {
		t.Fatalf("error should carry git's explanation, got %q", err)
	}
}

func TestStderrSummary(t *testing.T) {
	stderr := `To example.test:repo.git
 ! [rejected]        main -> main (fetch first)
error: failed to push some refs to 'example.test:repo.git'
hint: Updates were rejected because the remote contains work that you do
hint: not have locally.
`
	if got, want := stderrSummary(stderr), "error: failed to push some refs to 'example.test:repo.git'"; got != want {
		t.Errorf("stderrSummary = %q, want %q", got, want)
	}
	if got := stderrSummary("hint: only hints\n"); got != "" {
		t.Errorf("stderrSummary of hints only = %q, want empty", got)
	}
}

func TestRunTimeoutKillsHelperProcesses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process groups are unix-only")
	}
	dir := t.TempDir()
	gitCmd(t, dir, "init")
	// A git alias that starts a long-lived helper holding git's output pipes,
	// like ssh during a stalled fetch.
	gitCmd(t, dir, "config", "alias.stall", "!sleep 30 & sleep 30")

	saved := localTimeout
	localTimeout = 200 * time.Millisecond
	defer func() { localTimeout = saved }()

	start := time.Now()
	_, err := run(dir, "stall")
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("err = %v, want timeout", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("run returned after %s; helper processes kept it waiting", elapsed)
	}
}
