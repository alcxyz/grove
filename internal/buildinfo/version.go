package buildinfo

import (
	"runtime/debug"
	"strings"
)

// Resolve returns an injected package version, or derives an identifiable
// development version from Go's embedded VCS metadata for ordinary go builds.
func Resolve(configured string) string {
	info, _ := debug.ReadBuildInfo()
	return resolve(configured, info)
}

func resolve(configured string, info *debug.BuildInfo) string {
	if configured != "" && configured != "dev" {
		return configured
	}

	revision, modified := vcsRevision(info)
	if len(revision) < 12 || !isHex(revision) {
		return "dev"
	}

	resolved := "dev-" + strings.ToLower(revision[:12])
	if modified {
		resolved += "-dirty"
	}
	return resolved
}

func vcsRevision(info *debug.BuildInfo) (string, bool) {
	if info == nil {
		return "", false
	}

	var revision string
	var modified bool
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.modified":
			modified = setting.Value == "true"
		}
	}
	return revision, modified
}

func isHex(value string) bool {
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return value != ""
}
