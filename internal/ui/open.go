package ui

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

// launcherWait bounds how long OpenURL waits for the launcher's exit status.
var launcherWait = 10 * time.Second

// OpenURL opens url in the default browser with open on macOS or xdg-open
// elsewhere, and reports the launcher's exit status. Some launchers wait for
// the browser to exit, so after launcherWait OpenURL stops waiting and treats
// the launch as successful; a late failure would only be a stale message.
// Output is discarded rather than piped, so a browser that outlives the
// launcher never writes to a closed pipe.
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
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			return fmt.Errorf("%s: %w", launcher, err)
		}
	case <-time.After(launcherWait):
	}
	return nil
}
