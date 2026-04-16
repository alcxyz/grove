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
	Name  string `yaml:"name"`
	Match string `yaml:"match"` // prefix/substring match on repo name
}

type Config struct {
	BasePath        string   `yaml:"base_path"`
	Org             string   `yaml:"org"`
	Prefixes        []string `yaml:"prefixes"`
	Groups          []Group  `yaml:"groups"`
	RefreshSecs     int      `yaml:"refresh_secs"`
	ScreensaverSecs int      `yaml:"screensaver_secs"`
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
		return cfg
	}

	_ = yaml.Unmarshal(data, &cfg)

	if cfg.Org == "" {
		cfg.Org = Default.Org
	}
	if len(cfg.Prefixes) == 0 {
		cfg.Prefixes = Default.Prefixes
	}
	if cfg.RefreshSecs <= 0 {
		cfg.RefreshSecs = Default.RefreshSecs
	}
	if cfg.ScreensaverSecs < 0 {
		cfg.ScreensaverSecs = 0 // 0 = disabled
	}

	// Expand ~ in base_path
	if len(cfg.BasePath) > 0 && cfg.BasePath[0] == '~' {
		home, _ := os.UserHomeDir()
		cfg.BasePath = filepath.Join(home, cfg.BasePath[1:])
	}

	return cfg
}

// ResolveGroups returns the effective groups to use.  If none are configured
// it derives one group per unique prefix from the prefixes list.
func (c Config) ResolveGroups() []Group {
	if len(c.Groups) > 0 {
		return c.Groups
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

// GroupFor returns the group name for a given repo name.
// Returns "other" if no group matches.
func (c Config) GroupFor(repoName string) string {
	for _, g := range c.ResolveGroups() {
		if strings.HasPrefix(repoName, g.Match) || strings.Contains(repoName, g.Match) {
			return g.Name
		}
	}
	return "other"
}

// CacheKey returns a stable string that identifies the shape of this config.
// If org or prefixes change, the key changes and cached data is invalidated.
func (c Config) CacheKey() string {
	prefixes := make([]string, len(c.Prefixes))
	copy(prefixes, c.Prefixes)
	sort.Strings(prefixes)
	return c.Org + "|" + strings.Join(prefixes, ",")
}

// GroupOrder returns the position of a group name in the configured groups
// list.  Unknown groups sort last (9999).
func (c Config) GroupOrder(name string) int {
	for i, g := range c.ResolveGroups() {
		if g.Name == name {
			return i
		}
	}
	return 9999
}
