// Package cache provides disk-backed persistence for GitHub API responses.
//
// Each data type (PRs, branches, activity, CI runs) is stored as a single JSON
// file under $XDG_CACHE_HOME/grove/.  Every file wraps its payload in an
// envelope that records when it was written (cached_at) and a config-derived
// key (config_key).  If the key no longer matches the running config — because
// the user changed their profile owner or prefixes — the data is discarded and
// the caller must re-fetch.
//
// The cache is read synchronously at startup so the TUI can render immediately
// with last-known state.  Writes happen asynchronously (go cache.SaveX(...)) so
// they never block the UI event loop.
package cache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/alcxyz/grove/internal/model"
)

// Maximum entries stored per cache file.  These cap file size while staying
// well above what the app can actually display.
const (
	maxPRs      = 500
	maxBranches = 2000
	maxActivity = 100
	maxRuns     = 500
)

// ErrConfigChanged is returned by Load* when the stored config key doesn't
// match the current one — the caller should treat it as a cache miss.
var ErrConfigChanged = errors.New("cache: config changed")

type entry[T any] struct {
	CachedAt  time.Time `json:"cached_at"`
	ConfigKey string    `json:"config_key"`
	Data      T         `json:"data"`
}

func load[T any](dir, name, configKey string) (T, time.Time, error) {
	var e entry[T]
	data, err := os.ReadFile(filepath.Join(dir, name+".json"))
	if err != nil {
		return e.Data, time.Time{}, err
	}
	if err := json.Unmarshal(data, &e); err != nil {
		// Corrupt file — treat as miss so it gets overwritten on next save
		return e.Data, time.Time{}, err
	}
	if e.ConfigKey != configKey {
		return e.Data, time.Time{}, ErrConfigChanged
	}
	return e.Data, e.CachedAt, nil
}

func save[T any](dir, name, configKey string, data T) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(entry[T]{
		CachedAt:  time.Now(),
		ConfigKey: configKey,
		Data:      data,
	})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name+".json"), b, 0o644)
}

func capSlice[T any](s []T, max int) []T {
	if len(s) > max {
		return s[:max]
	}
	return s
}

func LoadPRs(dir, configKey string) ([]model.PR, time.Time, error) {
	return load[[]model.PR](dir, "prs", configKey)
}

func SavePRs(dir, configKey string, data []model.PR) error {
	return save(dir, "prs", configKey, capSlice(data, maxPRs))
}

func LoadBranches(dir, configKey string) ([]model.BranchInfo, time.Time, error) {
	return load[[]model.BranchInfo](dir, "branches", configKey)
}

func SaveBranches(dir, configKey string, data []model.BranchInfo) error {
	return save(dir, "branches", configKey, capSlice(data, maxBranches))
}

func LoadActivity(dir, configKey string) ([]model.Commit, time.Time, error) {
	return load[[]model.Commit](dir, "activity", configKey)
}

func SaveActivity(dir, configKey string, data []model.Commit) error {
	return save(dir, "activity", configKey, capSlice(data, maxActivity))
}

func LoadRuns(dir, configKey string) ([]model.WorkflowRun, time.Time, error) {
	return load[[]model.WorkflowRun](dir, "runs", configKey)
}

func SaveRuns(dir, configKey string, data []model.WorkflowRun) error {
	return save(dir, "runs", configKey, capSlice(data, maxRuns))
}
