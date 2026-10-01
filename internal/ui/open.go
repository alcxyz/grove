package ui

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
)

// OpenURL opens url in the default browser: open on macOS, xdg-open
// elsewhere. The launcher runs in the background and is reaped when it exits.
func OpenURL(url string) error {
	if url == "" {
		return errors.New("no URL available")
	}
	launcher := "xdg-open"
	if runtime.GOOS == "darwin" {
		launcher = "open"
	}
	cmd := exec.Command(launcher, url)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s: %w", launcher, err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
