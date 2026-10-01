//go:build unix

package git

import (
	"os/exec"
	"syscall"
)

// detachFromTerminal starts cmd in a new session without a controlling
// terminal, so git, ssh, and credential helpers cannot prompt over the TUI.
func detachFromTerminal(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
