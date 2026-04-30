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
}

// Profile holds per-profile configuration.
type Profile struct {
	Name      string   `yaml:"name"`
	Owner     string   `yaml:"owner"` // GitHub org or username; "" = no GitHub
	Forge     string   `yaml:"forge"` // "github" (default), "forgejo", etc.
	BasePaths []string `yaml:"base_paths"`
	BasePath  string   `yaml:"base_path"` // legacy; merged into BasePaths on load
	Prefixes  []string `yaml:"prefixes"`
	Groups    []Group  `yaml:"groups"`
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
		for j, path := range p.BasePaths {
			if len(path) > 0 && path[0] == '~' {
				p.BasePaths[j] = filepath.Join(home, path[1:])
			}
		}
		for j, g := range p.Groups {
			if len(g.BasePath) > 0 && g.BasePath[0] == '~' {
				p.Groups[j].BasePath = filepath.Join(home, g.BasePath[1:])
			}
			if len(g.MatchPath) > 0 && g.MatchPath[0] == '~' {
				p.Groups[j].MatchPath = filepath.Join(home, g.MatchPath[1:])
			}
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
		parts = append(parts, p.Owner+"|"+strings.Join(prefixes, ","))
	}
	return strings.Join(parts, ";")
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
