package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alcxyz/grove/internal/cache"
	"github.com/alcxyz/grove/internal/config"
	"github.com/alcxyz/grove/internal/model"
)

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
		runClone(cfg)
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
	for pi := range cfg.Profiles {
		p := &cfg.Profiles[pi]
		if len(p.BasePaths) == 0 {
			abs, _ := filepath.Abs(".")
			p.BasePaths = []string{abs}
		}
		for i, path := range p.BasePaths {
			abs, err := filepath.Abs(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error resolving path %q: %v\n", path, err)
				os.Exit(1)
			}
			info, err := os.Stat(abs)
			if err != nil || !info.IsDir() {
				fmt.Fprintf(os.Stderr, "Error: %q is not a valid directory\n", abs)
				os.Exit(1)
			}
			p.BasePaths[i] = abs
		}
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

	// Pre-load cached data so the app opens instantly with last-known state.
	var initPRs []model.PR
	var initPRsAt time.Time
	var initBranches []model.BranchInfo
	var initBranchesAt time.Time
	var initActivity []model.Commit
	var initActivityAt time.Time
	var initRuns []model.WorkflowRun
	var initRunsAt time.Time

	cacheKey := cfg.CacheKey()

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

	m := appModel{
		cfg:              cfg,
		statusMsg:        initStatus,
		loading:          true,
		scrollOffset:     map[tab]int{},
		tabSort:          map[tab]tabSortState{},
		cycleIdx:         -1,
		logPath:          logPath,
		grouped:          true,
		autoRefresh:      true,
		lastActivity:     time.Now(),
		ssDX:             1,
		ssDY:             1,
		cacheDir:         cacheDir,
		cacheKey:         cacheKey,
		prs:              initPRs,
		prsLoadedAt:      initPRsAt,
		branches:         initBranches,
		branchesLoadedAt: initBranchesAt,
		activity:         initActivity,
		activityLoadedAt: initActivityAt,
		runs:             initRuns,
		runsLoadedAt:     initRunsAt,
		activeProfile:    0,
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
