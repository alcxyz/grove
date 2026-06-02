package app

import (
	"encoding/json"
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
	dependencyGHAuth   = func() error {
		cmd := exec.Command("gh", "auth", "status", "-h", "github.com")
		return cmd.Run()
	}
	dependencyTeaLogins = func() ([]dependencyTeaLogin, error) {
		cmd := exec.Command("tea", "logins", "list", "--output", "json")
		out, err := cmd.Output()
		if err != nil {
			return nil, err
		}
		var logins []dependencyTeaLogin
		if err := json.Unmarshal(out, &logins); err != nil {
			return nil, err
		}
		return logins, nil
	}
	dependencyAzureDevOps = func() error {
		cmd := exec.Command("az", "repos", "--help")
		return cmd.Run()
	}
)

type dependencyTeaLogin struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// DependencyWarnings returns startup warnings for required provider tooling and
// credentials. GitHub can use direct token auth or the gh CLI. Forgejo can use
// direct token auth or the tea CLI, depending on each resolved remote's
// auth_mode.
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
	needsGitHubCLIAuth := false
	needsForgejoTeaAuth := false
	forgejoTeaURLs := map[string]struct{}{}
	needsAzureCLI := false
	for _, remote := range cfg.AllRemotes() {
		if remote.Owner == "" {
			continue
		}
		switch remote.EffectiveForge() {
		case "github":
			switch strings.ToLower(remote.EffectiveAuthMode()) {
			case "gh":
				needsGitHubCLIAuth = true
			case "token":
				if remote.TokenFile == "" {
					needsGitHubEnvToken = true
				} else {
					checkGitHubRemoteDependency(remote, add)
				}
			default:
				add(fmt.Sprintf("%s unsupported auth_mode %q; use token or gh", dependencyRemoteLabel(remote), remote.AuthMode))
			}
		case "forgejo":
			switch strings.ToLower(remote.EffectiveAuthMode()) {
			case "tea":
				needsForgejoTeaAuth = true
				if remote.InstanceURL == "" {
					add(fmt.Sprintf("%s missing instance_url", dependencyRemoteLabel(remote)))
				} else {
					forgejoTeaURLs[normaliseDependencyURL(remote.InstanceURL)] = struct{}{}
				}
			case "token":
				checkForgejoRemoteDependency(remote, add)
			default:
				add(fmt.Sprintf("%s unsupported auth_mode %q; use token or tea", dependencyRemoteLabel(remote), remote.AuthMode))
			}
		case "azuredevops":
			needsAzureCLI = true
			if strings.TrimSpace(remote.Project) == "" {
				add(fmt.Sprintf("%s missing project", dependencyRemoteLabel(remote)))
			}
		}
	}

	if needsGitHubCLIAuth {
		if _, err := dependencyLookPath("gh"); err != nil {
			add("gh not found on PATH; GitHub remotes with auth_mode: gh will fail")
		} else if err := dependencyGHAuth(); err != nil {
			add("gh is not authenticated for github.com; run gh auth login")
		}
	}
	if needsForgejoTeaAuth {
		if _, err := dependencyLookPath("tea"); err != nil {
			add("tea not found on PATH; Forgejo remotes with auth_mode: tea will fail")
		} else {
			checkForgejoTeaDependency(forgejoTeaURLs, add)
		}
	}

	if needsGitHubEnvToken && !dependencyGitHubEnvTokenConfigured() {
		add("GitHub token not configured; public API calls may work, but private repos and rate limits may fail")
	}
	if needsAzureCLI {
		if _, err := dependencyLookPath("az"); err != nil {
			add("az not found on PATH; Azure DevOps remotes use Azure CLI authentication and will fail")
		} else if err := dependencyAzureDevOps(); err != nil {
			add("az repos unavailable; install or enable the azure-devops Azure CLI extension")
		}
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

func checkForgejoTeaDependency(requiredURLs map[string]struct{}, add func(string)) {
	logins, err := dependencyTeaLogins()
	if err != nil {
		add(fmt.Sprintf("tea login status unavailable; run tea logins add: %v", err))
		return
	}
	if len(logins) == 0 {
		add("tea is not logged in to any Forgejo/Gitea instance; run tea logins add")
		return
	}
	available := make(map[string]struct{}, len(logins))
	for _, login := range logins {
		if login.URL != "" {
			available[normaliseDependencyURL(login.URL)] = struct{}{}
		}
	}
	for url := range requiredURLs {
		if _, ok := available[url]; !ok {
			add(fmt.Sprintf("tea has no login for %s; run tea logins add", url))
		}
	}
}

func normaliseDependencyURL(url string) string {
	return strings.TrimRight(strings.ToLower(strings.TrimSpace(url)), "/")
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
