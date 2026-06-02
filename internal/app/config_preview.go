package app

import (
	"fmt"
	"os"
	"strings"

	"github.com/alcxyz/grove/internal/config"
	"github.com/alcxyz/grove/internal/model"
)

func (m Model) openProfileConfigPreview() Model {
	m.showConfigPreview = true
	m.configScroll = 0
	m.configPreview = m.renderActiveProfileConfigPreview()
	return m
}

func (m Model) openRepoConfigPreview(repo model.Repo) Model {
	m.showConfigPreview = true
	m.configScroll = 0
	m.configPreview = m.renderRepoConfigPreview(repo)
	return m
}

func (m Model) renderActiveProfileConfigPreview() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Config preview\n")
	fmt.Fprintf(&b, "Path: %s\n", config.ConfigPath())
	fmt.Fprintf(&b, "Refresh: %ds\n", m.cfg.RefreshSecs)
	if m.cfg.ScreensaverSecs == 0 {
		fmt.Fprintf(&b, "Screensaver: disabled\n")
	} else {
		fmt.Fprintf(&b, "Screensaver: %ds\n", m.cfg.ScreensaverSecs)
	}

	profiles := m.cfg.Profiles
	if m.activeProfile >= 0 && m.activeProfile < len(m.cfg.Profiles) {
		profiles = []config.Profile{m.cfg.Profiles[m.activeProfile]}
	} else if m.activeProfile == -1 {
		fmt.Fprintf(&b, "\nActive profile: All (%d profiles)\n", len(m.cfg.Profiles))
	}

	for i, p := range profiles {
		if i > 0 {
			b.WriteString("\n")
		}
		writeProfilePreview(&b, p)
	}
	return b.String()
}

func (m Model) renderRepoConfigPreview(repo model.Repo) string {
	p, ok := profileByName(m.cfg.Profiles, repo.Profile)
	if !ok {
		return fmt.Sprintf("Config preview\n\nRepo: %s\nProfile: %s not found\n", repo.Name, repo.Profile)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Config preview: %s\n", repo.Name)
	fmt.Fprintf(&b, "Path: %s\n", repo.Path)
	fmt.Fprintf(&b, "Profile: %s\n", p.Name)
	fmt.Fprintf(&b, "Group: %s\n", p.GroupFor(repo.Name, repo.Path))
	if ov, ok := repoOverrideForPreview(p, repo.Name); ok {
		fmt.Fprintf(&b, "Repo override: yes (%s)\n", ov.Name)
	} else {
		fmt.Fprintf(&b, "Repo override: no\n")
	}

	b.WriteString("\nResolved remotes\n")
	writeRemotePreview(&b, "code", p.CodeRemote(repo.Name, repo.Path), repo.Name)
	writeRemotePreview(&b, "social", p.SocialRemote(repo.Name, repo.Path), repo.Name)
	writeRemotePreview(&b, "ci", p.CIRemote(repo.Name, repo.Path), repo.Name)

	b.WriteString("\nMatching group\n")
	if g, ok := groupOverrideForPreview(p, repo.Name, repo.Path); ok {
		writeGroupPreview(&b, g)
	} else {
		b.WriteString("  none\n")
	}

	return b.String()
}

func writeProfilePreview(b *strings.Builder, p config.Profile) {
	fmt.Fprintf(b, "\nProfile: %s\n", p.Name)
	fmt.Fprintf(b, "Base paths: %s\n", joinOrNone(p.BasePaths))
	fmt.Fprintf(b, "Repo paths: %s\n", joinOrNone(p.RepoPaths))
	fmt.Fprintf(b, "Exclude paths: %s\n", joinOrNone(p.ExcludePaths))
	fmt.Fprintf(b, "Exclude repos: %s\n", joinOrNone(p.ExcludeRepos))
	fmt.Fprintf(b, "Prefixes: %s\n", joinOrNone(p.Prefixes))

	b.WriteString("\nDefault remotes\n")
	writeRemotePreview(b, "code", p.CodeRemote("", ""), "")
	writeRemotePreview(b, "social", p.SocialRemote("", ""), "")
	writeRemotePreview(b, "ci", p.CIRemote("", ""), "")

	b.WriteString("\nGroups\n")
	if len(p.Groups) == 0 {
		b.WriteString("  none\n")
	} else {
		for _, g := range p.Groups {
			writeGroupPreview(b, g)
		}
	}

	b.WriteString("\nRepo overrides\n")
	if len(p.Repos) == 0 {
		b.WriteString("  none\n")
	} else {
		for _, r := range p.Repos {
			fmt.Fprintf(b, "  - %s\n", r.Name)
			writeRemoteOverridePreview(b, "code", r.Code)
			writeRemoteOverridePreview(b, "social", r.Social)
			writeRemoteOverridePreview(b, "ci", r.CI)
		}
	}
}

func writeGroupPreview(b *strings.Builder, g config.Group) {
	fmt.Fprintf(b, "  - %s\n", g.Name)
	if g.Match != "" {
		fmt.Fprintf(b, "    match: %s\n", g.Match)
	}
	if g.MatchPath != "" {
		fmt.Fprintf(b, "    match_path: %s\n", g.MatchPath)
	}
	if g.BasePath != "" {
		fmt.Fprintf(b, "    clone base_path: %s\n", g.BasePath)
	}
	writeRemoteOverridePreview(b, "code", g.Code)
	writeRemoteOverridePreview(b, "social", g.Social)
	writeRemoteOverridePreview(b, "ci", g.CI)
}

func writeRemotePreview(b *strings.Builder, label string, r config.Remote, localName string) {
	if r.Owner == "" {
		fmt.Fprintf(b, "  %s: disabled\n", label)
		return
	}
	target := r.Owner
	if localName != "" || r.Repo != "" {
		target = r.FullName(localName)
	}
	fmt.Fprintf(b, "  %s: %s\n", label, target)
	fmt.Fprintf(b, "    forge: %s\n", r.EffectiveForge())
	if r.InstanceURL != "" {
		fmt.Fprintf(b, "    instance: %s\n", r.InstanceURL)
	}
	if r.TokenFile != "" {
		fmt.Fprintf(b, "    token_file: %s\n", compactHome(r.TokenFile))
	}
	if r.AuthMode != "" {
		fmt.Fprintf(b, "    auth_mode: %s\n", r.AuthMode)
	}
	if r.CloneProto != "" {
		fmt.Fprintf(b, "    clone_proto: %s\n", r.CloneProto)
	}
	if r.SSHHost != "" {
		fmt.Fprintf(b, "    ssh_host: %s\n", r.SSHHost)
	}
	if r.Repo != "" && localName != "" && r.Repo != localName {
		fmt.Fprintf(b, "    local repo: %s\n", localName)
	}
}

func writeRemoteOverridePreview(b *strings.Builder, label string, r config.Remote) {
	if r.Key() == "|||||||" {
		return
	}
	parts := remoteOverrideParts(r)
	if len(parts) == 0 {
		return
	}
	fmt.Fprintf(b, "    %s override: %s\n", label, strings.Join(parts, ", "))
}

func remoteOverrideParts(r config.Remote) []string {
	var parts []string
	if r.Owner != "" {
		parts = append(parts, "owner="+r.Owner)
	}
	if r.Repo != "" {
		parts = append(parts, "repo="+r.Repo)
	}
	if r.Forge != "" {
		parts = append(parts, "forge="+r.Forge)
	}
	if r.InstanceURL != "" {
		parts = append(parts, "instance="+r.InstanceURL)
	}
	if r.TokenFile != "" {
		parts = append(parts, "token_file="+compactHome(r.TokenFile))
	}
	if r.AuthMode != "" {
		parts = append(parts, "auth_mode="+r.AuthMode)
	}
	if r.CloneProto != "" {
		parts = append(parts, "clone_proto="+r.CloneProto)
	}
	if r.SSHHost != "" {
		parts = append(parts, "ssh_host="+r.SSHHost)
	}
	return parts
}

func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}

func compactHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == home {
		return "~"
	}
	if strings.HasPrefix(path, home+"/") {
		return "~/" + strings.TrimPrefix(path, home+"/")
	}
	return path
}

func repoOverrideForPreview(p config.Profile, repoName string) (config.RepoOverride, bool) {
	for _, r := range p.Repos {
		if r.Name == repoName {
			return r, true
		}
	}
	return config.RepoOverride{}, false
}

func groupOverrideForPreview(p config.Profile, repoName, repoPath string) (config.Group, bool) {
	for _, g := range p.ResolveGroups() {
		if g.MatchPath != "" && strings.HasPrefix(repoPath, g.MatchPath) {
			return g, true
		}
		if g.Match != "" && (strings.HasPrefix(repoName, g.Match) || strings.Contains(repoName, g.Match)) {
			return g, true
		}
	}
	return config.Group{}, false
}
