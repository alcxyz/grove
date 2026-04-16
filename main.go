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

	cfg := config.Load()

	// CLI arg overrides config
	if len(os.Args) > 1 {
		cfg.BasePath = os.Args[1]
	}

	// Resolve to absolute
	if cfg.BasePath != "" {
		abs, err := filepath.Abs(cfg.BasePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving path %q: %v\n", cfg.BasePath, err)
			os.Exit(1)
		}
		cfg.BasePath = abs
	} else {
		abs, _ := filepath.Abs(".")
		cfg.BasePath = abs
	}

	info, err := os.Stat(cfg.BasePath)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: %q is not a valid directory\n", cfg.BasePath)
		os.Exit(1)
	}

	initStatus := fmt.Sprintf("Scanning %s...", cfg.BasePath)
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
	}

	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
