package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alcxyz/grove/internal/app"
	"github.com/alcxyz/grove/internal/cache"
	"github.com/alcxyz/grove/internal/clone"
	"github.com/alcxyz/grove/internal/config"
	"github.com/alcxyz/grove/internal/model"
)

// version is injected at build time via -ldflags "-X main.version=<tag>".
// Falls back to "dev" for local builds.
var version = "dev"

//go:embed config.example.yaml
var exampleConfig []byte

func setupLog() (string, func()) {
	logPath := config.LogPath()
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return "", func() {}
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return "", func() {}
	}
	log.SetOutput(f)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	return logPath, func() { f.Close() }
}

func main() {
	logPath, closeLog := setupLog()
	defer closeLog()

	// First-run bootstrap: write the example config to the XDG path so the
	// user has a real file to edit rather than relying on compiled defaults.
	var bootstrapMsg string
	if config.NeedsBootstrap() {
		if p, err := config.BootstrapXDG(exampleConfig); err != nil {
			log.Printf("bootstrap config: %v", err)
		} else {
			bootstrapMsg = fmt.Sprintf("Created config at %s — edit it to customise.", p)
			log.Printf("bootstrapped config at %s", p)
		}
	}

	// Version flag — print and exit before any TUI setup.
	if len(os.Args) > 1 && (os.Args[1] == "-v" || os.Args[1] == "--version" || os.Args[1] == "version") {
		fmt.Printf("grove %s\nconfig: %s\ncache:  %s\nlog:    %s\n",
			version, config.ConfigPath(), config.CacheDir(), config.LogPath())
		return
	}

	cfg := config.Load()

	// clone subcommand: enumerate org repos and clone any that are missing.
	if len(os.Args) > 1 && os.Args[1] == "clone" {
		clone.Run(cfg)
		return
	}

	// CLI path args override the first profile's base paths.
	if len(os.Args) > 1 {
		paths := os.Args[1:]
		home, _ := os.UserHomeDir()
		for i, p := range paths {
			if len(p) > 0 && p[0] == '~' {
				paths[i] = filepath.Join(home, p[1:])
			}
		}
		if len(cfg.Profiles) > 0 {
			cfg.Profiles[0].BasePaths = paths
			cfg.Profiles = cfg.Profiles[:1]
		}
	}

	// Resolve all profile paths to absolute and validate.
	// Invalid paths are warned about and skipped rather than being fatal,
	// so that a config used across machines (with different path layouts)
	// doesn't prevent the app from starting.
	for pi := range cfg.Profiles {
		p := &cfg.Profiles[pi]
		if len(p.BasePaths) == 0 {
			abs, _ := filepath.Abs(".")
			p.BasePaths = []string{abs}
		}
		valid := p.BasePaths[:0]
		for _, path := range p.BasePaths {
			abs, err := filepath.Abs(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: skipping path %q: %v\n", path, err)
				continue
			}
			info, err := os.Stat(abs)
			if err != nil || !info.IsDir() {
				fmt.Fprintf(os.Stderr, "Warning: skipping %q (not a valid directory)\n", abs)
				continue
			}
			valid = append(valid, abs)
		}
		if len(valid) == 0 {
			abs, _ := filepath.Abs(".")
			fmt.Fprintf(os.Stderr, "Warning: profile %q has no valid paths, falling back to %s\n", p.Name, abs)
			valid = []string{abs}
		}
		p.BasePaths = valid
	}

	var initStatus string
	if len(cfg.Profiles) == 1 {
		if len(cfg.Profiles[0].BasePaths) > 0 {
			initStatus = fmt.Sprintf("Scanning %s...", cfg.Profiles[0].BasePaths[0])
		} else {
			initStatus = "Scanning..."
		}
	} else {
		initStatus = fmt.Sprintf("Scanning %d profiles...", len(cfg.Profiles))
	}
	if bootstrapMsg != "" {
		initStatus = bootstrapMsg
	}

	cacheDir := config.CacheDir()
	cacheKey := cfg.CacheKey()

	// Pre-load cached data so the app opens instantly with last-known state.
	var initPRs []model.PR
	var initPRsAt time.Time
	var initBranches []model.BranchInfo
	var initBranchesAt time.Time
	var initActivity []model.Commit
	var initActivityAt time.Time
	var initRuns []model.WorkflowRun
	var initRunsAt time.Time
	var initIssues []model.Issue
	var initIssuesAt time.Time

	if prs, at, err := cache.LoadPRs(cacheDir, cacheKey); err == nil {
		initPRs, initPRsAt = prs, at
	}
	if branches, at, err := cache.LoadBranches(cacheDir, cacheKey); err == nil {
		initBranches, initBranchesAt = branches, at
	}
	if activity, at, err := cache.LoadActivity(cacheDir, cacheKey); err == nil {
		initActivity, initActivityAt = activity, at
	}
	if runs, at, err := cache.LoadRuns(cacheDir, cacheKey); err == nil {
		initRuns, initRunsAt = runs, at
	}
	if issues, at, err := cache.LoadIssues(cacheDir, cacheKey); err == nil {
		initIssues, initIssuesAt = issues, at
	}

	// Restore last active profile from persisted state.
	initProfile := 0
	if st, err := cache.LoadState(cacheDir); err == nil {
		if st.ActiveProfile >= -1 && st.ActiveProfile < len(cfg.Profiles) {
			initProfile = st.ActiveProfile
		}
	}

	m := app.New(app.Options{
		Cfg:              cfg,
		Version:          version,
		StatusMsg:        initStatus,
		LogPath:          logPath,
		CacheDir:         cacheDir,
		CacheKey:         cacheKey,
		PRs:              initPRs,
		PRsLoadedAt:      initPRsAt,
		Branches:         initBranches,
		BranchesLoadedAt: initBranchesAt,
		Activity:         initActivity,
		ActivityLoadedAt: initActivityAt,
		Runs:             initRuns,
		RunsLoadedAt:     initRunsAt,
		Issues:           initIssues,
		IssuesLoadedAt:   initIssuesAt,
		ActiveProfile:    initProfile,
	})

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
