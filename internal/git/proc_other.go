//go:build !unix

package git

import "os/exec"

func detachFromTerminal(*exec.Cmd) {}
