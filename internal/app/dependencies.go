package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/alcxyz/grove/internal/config"
)

var (
	dependencyLookPath = exec.LookPath
	dependencyReadFile = os.ReadFile
	dependencyGetenv   = os.Getenv
)

// DependencyWarnings returns startup warnings for required provider tooling and
// credentials. Forgejo and GitHub use direct HTTP token auth, so neither
// forgejo-cli nor gh is a Grove provider dependency.
func DependencyWarnings(cfg config.Config) []string {
	var warnings []string
	seen := map[string]struct{}{}
	add := func(msg string) {
		if _, ok := seen[msg]; ok {
			return
		}
		seen[msg] = struct{}{}
		warnings = append(warnings, msg)
	}

	if _, err := dependencyLookPath("git"); err != nil {
		add("git not found on PATH; local repo state, fetch, pull, and clone operations will fail")
	}

	needsGitHubEnvToken := false
	for _, remote := range cfg.AllRemotes() {
		if remote.Owner == "" {
			continue
		}
		switch remote.EffectiveForge() {
		case "github":
			if remote.TokenFile == "" {
				needsGitHubEnvToken = true
			} else {
				checkGitHubRemoteDependency(remote, add)
			}
		case "forgejo":
			checkForgejoRemoteDependency(remote, add)
		}
	}

	if needsGitHubEnvToken && !dependencyGitHubEnvTokenConfigured() {
		add("GitHub token not configured; public API calls may work, but private repos and rate limits may fail")
	}

	return warnings
}

func dependencyGitHubEnvTokenConfigured() bool {
	return strings.TrimSpace(dependencyGetenv("GH_TOKEN")) != "" ||
		strings.TrimSpace(dependencyGetenv("GITHUB_TOKEN")) != ""
}

func checkGitHubRemoteDependency(remote config.Remote, add func(string)) {
	if remote.TokenFile == "" {
		return
	}
	label := dependencyRemoteLabel(remote)
	data, err := dependencyReadFile(remote.TokenFile)
	if err != nil {
		add(fmt.Sprintf("%s token_file %s cannot be read: %v", label, remote.TokenFile, err))
		return
	}
	if strings.TrimSpace(string(data)) == "" {
		add(fmt.Sprintf("%s token_file %s is empty", label, remote.TokenFile))
	}
}

func checkForgejoRemoteDependency(remote config.Remote, add func(string)) {
	label := dependencyRemoteLabel(remote)
	if remote.InstanceURL == "" {
		add(fmt.Sprintf("%s missing instance_url", label))
	}
	if remote.TokenFile == "" {
		add(fmt.Sprintf("%s missing token_file; private Forgejo PRs, issues, branches, and CI may fail", label))
		return
	}
	data, err := dependencyReadFile(remote.TokenFile)
	if err != nil {
		add(fmt.Sprintf("%s token_file %s cannot be read: %v", label, remote.TokenFile, err))
		return
	}
	if strings.TrimSpace(string(data)) == "" {
		add(fmt.Sprintf("%s token_file %s is empty", label, remote.TokenFile))
	}
}

func dependencyRemoteLabel(remote config.Remote) string {
	parts := []string{remote.EffectiveForge()}
	if remote.Owner != "" {
		parts = append(parts, remote.Owner)
	}
	if remote.InstanceURL != "" {
		parts = append(parts, remote.InstanceURL)
	}
	return strings.Join(parts, " ")
}
