package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Group struct {
	Name      string `yaml:"name"`
	Match     string `yaml:"match"`      // prefix/substring match on repo name
	MatchPath string `yaml:"match_path"` // path prefix match on repo location
	BasePath  string `yaml:"base_path"`  // optional: clone destination for this group
	Code      Remote `yaml:"code"`
	Social    Remote `yaml:"social"`
	CI        Remote `yaml:"ci"`
}

// Remote describes how grove should talk to one forge concern.
type Remote struct {
	Owner       string `yaml:"owner"`        // org or username
	Repo        string `yaml:"repo"`         // optional remote repo name; defaults to local repo name
	Forge       string `yaml:"forge"`        // "github" (default), "forgejo", "azuredevops"
	InstanceURL string `yaml:"instance_url"` // base URL for non-GitHub forges
	Project     string `yaml:"project"`      // Azure DevOps project for azuredevops remotes
	TokenFile   string `yaml:"token_file"`   // path to file containing API token
	AuthMode    string `yaml:"auth_mode"`    // auth mode: "token" (default), "gh" for GitHub, "tea" for Forgejo
	CloneProto  string `yaml:"clone_proto"`  // "https" (default) or "ssh"
	SSHHost     string `yaml:"ssh_host"`     // optional SSH clone host when it differs from instance_url host
}

// RepoOverride applies concern-specific remote overrides to one repo by name.
type RepoOverride struct {
	Name   string `yaml:"name"`
	Code   Remote `yaml:"code"`
	Social Remote `yaml:"social"`
	CI     Remote `yaml:"ci"`
}

// Profile holds per-profile configuration.
type Profile struct {
	Name         string         `yaml:"name"`
	Owner        string         `yaml:"owner"`        // org or username; "" = no forge API
	Forge        string         `yaml:"forge"`        // "github" (default), "forgejo", "azuredevops"
	InstanceURL  string         `yaml:"instance_url"` // base URL for non-GitHub forges, e.g. "https://git.alc.xyz"
	Project      string         `yaml:"project"`      // Azure DevOps project for azuredevops profiles
	TokenFile    string         `yaml:"token_file"`   // path to file containing API token
	AuthMode     string         `yaml:"auth_mode"`    // auth mode: "token" (default), "gh" for GitHub, "tea" for Forgejo
	CloneProto   string         `yaml:"clone_proto"`  // "https" (default) or "ssh"
	SSHHost      string         `yaml:"ssh_host"`     // optional SSH clone host when it differs from instance_url host
	BasePaths    []string       `yaml:"base_paths"`
	BasePath     string         `yaml:"base_path"` // legacy; merged into BasePaths on load
	RepoPaths    []string       `yaml:"repo_paths"`
	ExcludePaths []string       `yaml:"exclude_paths"`
	ExcludeRepos []string       `yaml:"exclude_repos"`
	Prefixes     []string       `yaml:"prefixes"`
	Groups       []Group        `yaml:"groups"`
	Social       Remote         `yaml:"social"`
	CI           Remote         `yaml:"ci"`
	Repos        []RepoOverride `yaml:"repos"`
}

type Config struct {
	// Legacy flat fields — kept for YAML backward compat only.
	// Migrated into Profiles[0] by Load() when Profiles is empty.
	BasePath  string   `yaml:"base_path"`
	BasePaths []string `yaml:"base_paths"`
	Org       string   `yaml:"org"`
	Prefixes  []string `yaml:"prefixes"`
	Groups    []Group  `yaml:"groups"`

	Profiles        []Profile `yaml:"profiles"`
	RefreshSecs     int       `yaml:"refresh_secs"`
	ScreensaverSecs int       `yaml:"screensaver_secs"`
}

var Default = Config{
	Org:             "my-org",
	Prefixes:        []string{"service-", "platform-"},
	RefreshSecs:     300,
	ScreensaverSecs: 300,
}

// xdgDir returns the XDG base directory, falling back to the provided default
// relative to $HOME.
func xdgDir(envKey, homeRel string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, homeRel)
}

// xdgConfigPath returns the canonical XDG config file path regardless of
// whether it exists yet.
func xdgConfigPath() string {
	return filepath.Join(xdgDir("XDG_CONFIG_HOME", ".config"), "grove", "config.yaml")
}

// legacyConfigPath returns the old ~/.grove.yaml path.
func legacyConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".grove.yaml")
}

// ConfigPath returns the config file that should be read: XDG if it exists,
// legacy ~/.grove.yaml as a fallback.
func ConfigPath() string {
	xdg := xdgConfigPath()
	if _, err := os.Stat(xdg); err == nil {
		return xdg
	}
	return legacyConfigPath()
}

// LogPath returns the path for the runtime log file.
func LogPath() string {
	dir := filepath.Join(xdgDir("XDG_STATE_HOME", ".local/state"), "grove")
	return filepath.Join(dir, "grove.log")
}

// CacheDir returns the XDG cache directory for grove.
func CacheDir() string {
	return filepath.Join(xdgDir("XDG_CACHE_HOME", ".cache"), "grove")
}

// NeedsBootstrap returns true when no config file exists at either the XDG or
// legacy location — i.e. this is a first run.
func NeedsBootstrap() bool {
	if _, err := os.Stat(xdgConfigPath()); err == nil {
		return false
	}
	if _, err := os.Stat(legacyConfigPath()); err == nil {
		return false
	}
	return true
}

// BootstrapXDG writes example to the XDG config path, creating the directory
// if necessary.  Returns the path written so the caller can inform the user.
func BootstrapXDG(example []byte) (string, error) {
	p := xdgConfigPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(p, example, 0o644); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}
	return p, nil
}

func Load() Config {
	cfg := Default

	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		// No config file — synthesise default profile from Default fields.
		cfg.Profiles = []Profile{{
			Name:      "default",
			Owner:     cfg.Org,
			BasePaths: cfg.BasePaths,
			BasePath:  cfg.BasePath,
			Prefixes:  cfg.Prefixes,
			Groups:    cfg.Groups,
		}}
		normaliseProfiles(&cfg, true)
		return cfg
	}

	_ = yaml.Unmarshal(data, &cfg)

	if cfg.RefreshSecs <= 0 {
		cfg.RefreshSecs = Default.RefreshSecs
	}
	if cfg.ScreensaverSecs < 0 {
		cfg.ScreensaverSecs = 0 // 0 = disabled
	}

	// If no profiles defined, synthesize one from legacy flat fields.
	synthesized := len(cfg.Profiles) == 0
	if synthesized {
		org := cfg.Org
		if org == "" {
			org = Default.Org
		}
		prefixes := cfg.Prefixes
		if len(prefixes) == 0 {
			prefixes = Default.Prefixes
		}
		cfg.Profiles = []Profile{{
			Name:      "default",
			Owner:     org,
			BasePaths: cfg.BasePaths,
			BasePath:  cfg.BasePath,
			Prefixes:  prefixes,
			Groups:    cfg.Groups,
		}}
	}

	normaliseProfiles(&cfg, synthesized)

	return cfg
}

// normaliseProfiles normalises each profile: merge legacy base_path, expand ~,
// and (when fillPrefixDefaults is true) fill in default prefixes for profiles
// that have none. fillPrefixDefaults is true only for synthesized profiles;
// explicitly-defined profiles with empty prefixes match all repos.
func normaliseProfiles(cfg *Config, fillPrefixDefaults bool) {
	home, _ := os.UserHomeDir()
	for i := range cfg.Profiles {
		p := &cfg.Profiles[i]
		if p.BasePath != "" {
			p.BasePaths = append([]string{p.BasePath}, p.BasePaths...)
			p.BasePath = ""
		}
		p.TokenFile = expandHome(p.TokenFile, home)
		expandRemoteHome(&p.Social, home)
		expandRemoteHome(&p.CI, home)
		for j, path := range p.BasePaths {
			p.BasePaths[j] = expandHome(path, home)
		}
		for j, path := range p.RepoPaths {
			p.RepoPaths[j] = expandHome(path, home)
		}
		for j, path := range p.ExcludePaths {
			p.ExcludePaths[j] = expandHome(path, home)
		}
		for j, g := range p.Groups {
			p.Groups[j].BasePath = expandHome(g.BasePath, home)
			p.Groups[j].MatchPath = expandHome(g.MatchPath, home)
			expandRemoteHome(&p.Groups[j].Code, home)
			expandRemoteHome(&p.Groups[j].Social, home)
			expandRemoteHome(&p.Groups[j].CI, home)
		}
		for j := range p.Repos {
			expandRemoteHome(&p.Repos[j].Code, home)
			expandRemoteHome(&p.Repos[j].Social, home)
			expandRemoteHome(&p.Repos[j].CI, home)
		}
		if fillPrefixDefaults && len(p.Prefixes) == 0 {
			if len(cfg.Prefixes) > 0 {
				p.Prefixes = cfg.Prefixes
			} else {
				p.Prefixes = Default.Prefixes
			}
		}
		if p.Name == "" {
			p.Name = fmt.Sprintf("profile %d", i+1)
		}
	}
}

func expandHome(path, home string) string {
	if len(path) > 0 && path[0] == '~' {
		return filepath.Join(home, path[1:])
	}
	return path
}

func expandRemoteHome(remote *Remote, home string) {
	remote.TokenFile = expandHome(remote.TokenFile, home)
}

// ResolveGroups returns the effective groups for the first profile (or derived
// from Default prefixes when no profiles exist).
func (c Config) ResolveGroups() []Group {
	if len(c.Profiles) > 0 {
		return c.Profiles[0].ResolveGroups()
	}
	groups := make([]Group, len(c.Prefixes))
	for i, p := range c.Prefixes {
		groups[i] = Group{
			Name:  strings.TrimSuffix(p, "-"),
			Match: p,
		}
	}
	return groups
}

// GroupFor returns the group name for a given repo name and path.
// Delegates to the first profile when profiles are defined.
func (c Config) GroupFor(repoName, repoPath string) string {
	if len(c.Profiles) > 0 {
		return c.Profiles[0].GroupFor(repoName, repoPath)
	}
	return "other"
}

// GroupOrder returns the position of a group name in the configured groups
// list.  Unknown groups sort last (9999).
func (c Config) GroupOrder(name string) int {
	if len(c.Profiles) > 0 {
		return c.Profiles[0].GroupOrder(name)
	}
	return 9999
}

// CacheKey returns a stable string that identifies the shape of this config.
// If org or prefixes change, the key changes and cached data is invalidated.
func (c Config) CacheKey() string {
	var parts []string
	for _, p := range c.Profiles {
		prefixes := make([]string, len(p.Prefixes))
		copy(prefixes, p.Prefixes)
		sort.Strings(prefixes)
		parts = append(parts, p.cacheKeyPart(strings.Join(prefixes, ",")))
	}
	return strings.Join(parts, ";")
}

func (p Profile) cacheKeyPart(prefixes string) string {
	repoPaths := append([]string(nil), p.RepoPaths...)
	sort.Strings(repoPaths)
	excludePaths := append([]string(nil), p.ExcludePaths...)
	sort.Strings(excludePaths)
	excludeRepos := append([]string(nil), p.ExcludeRepos...)
	sort.Strings(excludeRepos)
	var groupParts []string
	for _, g := range p.Groups {
		groupParts = append(groupParts, strings.Join([]string{
			g.Name,
			g.Match,
			g.MatchPath,
			g.Code.Key(),
			g.Social.Key(),
			g.CI.Key(),
		}, "|"))
	}
	sort.Strings(groupParts)
	var repoParts []string
	for _, r := range p.Repos {
		repoParts = append(repoParts, strings.Join([]string{
			r.Name,
			r.Code.Key(),
			r.Social.Key(),
			r.CI.Key(),
		}, "|"))
	}
	sort.Strings(repoParts)
	return strings.Join([]string{
		p.Name,
		p.codeDefaults().Key(),
		p.Social.Key(),
		p.CI.Key(),
		prefixes,
		strings.Join(repoPaths, ","),
		strings.Join(excludePaths, ","),
		strings.Join(excludeRepos, ","),
		strings.Join(groupParts, ","),
		strings.Join(repoParts, ","),
	}, "|")
}

// Profile methods

// ResolveGroups returns the effective groups for this profile.
func (p Profile) ResolveGroups() []Group {
	if len(p.Groups) > 0 {
		return p.Groups
	}
	groups := make([]Group, len(p.Prefixes))
	for i, prefix := range p.Prefixes {
		groups[i] = Group{
			Name:  strings.TrimSuffix(prefix, "-"),
			Match: prefix,
		}
	}
	return groups
}

// GroupFor returns the group name for a given repo name and filesystem path.
// A group matches if its match_path is a prefix of the repo path, or its
// match is a prefix/substring of the repo name. First match wins.
// Returns "other" if no group matches.
func (p Profile) GroupFor(repoName, repoPath string) string {
	for _, g := range p.ResolveGroups() {
		if g.MatchPath != "" && strings.HasPrefix(repoPath, g.MatchPath) {
			return g.Name
		}
		if g.Match != "" && (strings.HasPrefix(repoName, g.Match) || strings.Contains(repoName, g.Match)) {
			return g.Name
		}
	}
	return "other"
}

// ExcludesRepo returns true when a repo should be omitted from this profile.
func (p Profile) ExcludesRepo(repoName, repoPath string) bool {
	for _, name := range p.ExcludeRepos {
		if name == repoName {
			return true
		}
	}
	for _, path := range p.ExcludePaths {
		if pathWithin(repoPath, path) {
			return true
		}
	}
	return false
}

func pathWithin(path, prefix string) bool {
	if path == "" || prefix == "" {
		return false
	}
	path = filepath.Clean(path)
	prefix = filepath.Clean(prefix)
	return path == prefix || strings.HasPrefix(path, prefix+string(os.PathSeparator))
}

// GroupOrder returns the position of a group name in the configured groups
// list.  Unknown groups sort last (9999).
func (p Profile) GroupOrder(name string) int {
	for i, g := range p.ResolveGroups() {
		if g.Name == name {
			return i
		}
	}
	return 9999
}

func (r Remote) Key() string {
	return strings.Join([]string{
		r.Owner,
		r.Repo,
		r.EffectiveForge(),
		r.InstanceURL,
		r.Project,
		r.TokenFile,
		r.AuthMode,
		r.CloneProto,
		r.SSHHost,
	}, "|")
}

// RepoName returns the remote repository name for a local repo name.
func (r Remote) RepoName(localName string) string {
	if r.Repo != "" {
		return r.Repo
	}
	return localName
}

// FullName returns the owner/repo name used by forge APIs.
func (r Remote) FullName(localName string) string {
	return r.Owner + "/" + r.RepoName(localName)
}

// EffectiveForge returns the forge name after applying the default.
func (r Remote) EffectiveForge() string {
	if r.Forge == "" {
		return "github"
	}
	return r.Forge
}

// EffectiveAuthMode returns the GitHub auth mode after applying defaults.
func (r Remote) EffectiveAuthMode() string {
	if r.AuthMode == "" {
		return "token"
	}
	return r.AuthMode
}

func mergeRemote(base, override Remote) Remote {
	if override.Owner != "" {
		base.Owner = override.Owner
	}
	if override.Repo != "" {
		base.Repo = override.Repo
	}
	if override.Forge != "" {
		if base.EffectiveForge() != override.EffectiveForge() {
			base.InstanceURL = ""
			base.Project = ""
			base.TokenFile = ""
			base.AuthMode = ""
			base.CloneProto = ""
			base.SSHHost = ""
		}
		base.Forge = override.Forge
	}
	if override.InstanceURL != "" {
		base.InstanceURL = override.InstanceURL
	}
	if override.Project != "" {
		base.Project = override.Project
	}
	if override.TokenFile != "" {
		base.TokenFile = override.TokenFile
	}
	if override.AuthMode != "" {
		base.AuthMode = override.AuthMode
	}
	if override.CloneProto != "" {
		base.CloneProto = override.CloneProto
	}
	if override.SSHHost != "" {
		base.SSHHost = override.SSHHost
	}
	return base
}

func groupMatches(g Group, repoName, repoPath string) bool {
	if g.MatchPath != "" && strings.HasPrefix(repoPath, g.MatchPath) {
		return true
	}
	if g.Match != "" && (strings.HasPrefix(repoName, g.Match) || strings.Contains(repoName, g.Match)) {
		return true
	}
	return false
}

func (p Profile) codeDefaults() Remote {
	return Remote{
		Owner:       p.Owner,
		Forge:       p.Forge,
		InstanceURL: p.InstanceURL,
		Project:     p.Project,
		TokenFile:   p.TokenFile,
		AuthMode:    p.AuthMode,
		CloneProto:  p.CloneProto,
		SSHHost:     p.SSHHost,
	}
}

func (p Profile) repoOverride(repoName string) (RepoOverride, bool) {
	for _, r := range p.Repos {
		if r.Name == repoName {
			return r, true
		}
	}
	return RepoOverride{}, false
}

func (p Profile) groupOverride(repoName, repoPath string) (Group, bool) {
	for _, g := range p.ResolveGroups() {
		if groupMatches(g, repoName, repoPath) {
			return g, true
		}
	}
	return Group{}, false
}

// CodeRemote resolves the forge used for repo hosting, branches, commits, and clone.
func (p Profile) CodeRemote(repoName, repoPath string) Remote {
	remote := p.codeDefaults()
	if g, ok := p.groupOverride(repoName, repoPath); ok {
		remote = mergeRemote(remote, g.Code)
	}
	if ov, ok := p.repoOverride(repoName); ok {
		remote = mergeRemote(remote, ov.Code)
	}
	return remote
}

// SocialRemote resolves the forge used for pull requests and issues.
func (p Profile) SocialRemote(repoName, repoPath string) Remote {
	remote := mergeRemote(p.codeDefaults(), p.Social)
	if g, ok := p.groupOverride(repoName, repoPath); ok {
		remote = mergeRemote(remote, g.Social)
	}
	if ov, ok := p.repoOverride(repoName); ok {
		remote = mergeRemote(remote, ov.Social)
	}
	return remote
}

// CIRemote resolves the forge used for workflow / pipeline runs.
func (p Profile) CIRemote(repoName, repoPath string) Remote {
	remote := mergeRemote(p.codeDefaults(), p.CI)
	if g, ok := p.groupOverride(repoName, repoPath); ok {
		remote = mergeRemote(remote, g.CI)
	}
	if ov, ok := p.repoOverride(repoName); ok {
		remote = mergeRemote(remote, ov.CI)
	}
	return remote
}

// AllRemotes returns every resolved remote configuration referenced by this config.
func (c Config) AllRemotes() []Remote {
	seen := map[string]struct{}{}
	var remotes []Remote
	add := func(r Remote) {
		key := r.Key()
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		remotes = append(remotes, r)
	}
	for _, p := range c.Profiles {
		add(p.CodeRemote("", ""))
		add(p.SocialRemote("", ""))
		add(p.CIRemote("", ""))
		for _, g := range p.ResolveGroups() {
			add(mergeRemote(p.codeDefaults(), g.Code))
			add(mergeRemote(mergeRemote(p.codeDefaults(), p.Social), g.Social))
			add(mergeRemote(mergeRemote(p.codeDefaults(), p.CI), g.CI))
		}
		for _, ov := range p.Repos {
			add(p.CodeRemote(ov.Name, ""))
			add(p.SocialRemote(ov.Name, ""))
			add(p.CIRemote(ov.Name, ""))
		}
	}
	return remotes
}
