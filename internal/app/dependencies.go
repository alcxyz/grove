package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/alcxyz/grove/internal/config"
)

var (
	dependencyLookPath     = exec.LookPath
	dependencyReadFile     = os.ReadFile
	dependencyGHAuthStatus = func() error {
		return exec.Command("gh", "auth", "status", "-h", "github.com").Run()
	}
)

// DependencyWarnings returns startup warnings for required provider tooling and
// credentials. Forgejo uses direct HTTP token auth, so forgejo-cli is not a
// Grove dependency.
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

	usesGitHub := false
	for _, remote := range cfg.AllRemotes() {
		if remote.Owner == "" {
			continue
		}
		switch remote.EffectiveForge() {
		case "github":
			usesGitHub = true
		case "forgejo":
			checkForgejoRemoteDependency(remote, add)
		}
	}

	if usesGitHub {
		if _, err := dependencyLookPath("gh"); err != nil {
			add("gh not found on PATH; GitHub PRs, issues, branches, CI, and GitHub clone will fail")
		} else if err := dependencyGHAuthStatus(); err != nil {
			add("gh is not authenticated for github.com; run gh auth login")
		}
	}

	return warnings
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
