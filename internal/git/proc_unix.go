//go:build unix

package git

import (
	"os/exec"
	"syscall"
)

// detachFromTerminal starts cmd in a new session without a controlling
// terminal, so git, ssh, and credential helpers cannot prompt over the TUI.
// On timeout the whole session is killed, since helpers such as ssh would
// otherwise outlive git while holding its output pipes.
func detachFromTerminal(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
