package ui

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
)

// OpenURL opens url in the default browser with open on macOS or xdg-open
// elsewhere, and waits for the launcher so its exit status can be reported.
// Run it off the UI goroutine: some launchers wait for the browser to exit.
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
	if err := exec.Command(launcher, url).Run(); err != nil {
		return fmt.Errorf("%s: %w", launcher, err)
	}
	return nil
}
