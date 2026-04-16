package config

import (
	"os"
	"path/filepath"
	"testing"
)

// ── Profile.GroupFor ──────────────────────────────────────────────────────

func TestGroupFor_ExplicitGroups(t *testing.T) {
	p := Profile{
		Groups: []Group{
			{Name: "Services", Match: "service-"},
			{Name: "Platform", Match: "platform-"},
		},
	}
	cases := []struct {
		repo string
		want string
	}{
		{"service-api", "Services"},
		{"service-worker", "Services"},
		{"platform-core", "Platform"},
		{"infra-vpc", "other"},
		{"unrelated", "other"},
	}
	for _, c := range cases {
		if got := p.GroupFor(c.repo); got != c.want {
			t.Errorf("GroupFor(%q) = %q, want %q", c.repo, got, c.want)
		}
	}
}

func TestGroupFor_DerivedFromPrefixes(t *testing.T) {
	p := Profile{
		Prefixes: []string{"service-", "platform-"},
	}
	// Derived groups use the prefix minus trailing "-" as the name.
	if got := p.GroupFor("service-api"); got != "service" {
		t.Errorf("GroupFor(service-api) = %q, want service", got)
	}
	if got := p.GroupFor("platform-core"); got != "platform" {
		t.Errorf("GroupFor(platform-core) = %q, want platform", got)
	}
	if got := p.GroupFor("unknown"); got != "other" {
		t.Errorf("GroupFor(unknown) = %q, want other", got)
	}
}

func TestGroupFor_EmptyPrefixes(t *testing.T) {
	p := Profile{Prefixes: []string{}}
	// Empty prefixes + no groups → everything is "other"
	if got := p.GroupFor("anything"); got != "other" {
		t.Errorf("GroupFor(anything) = %q, want other", got)
	}
}

// ── Profile.GroupOrder ────────────────────────────────────────────────────

func TestGroupOrder(t *testing.T) {
	p := Profile{
		Groups: []Group{
			{Name: "A", Match: "a-"},
			{Name: "B", Match: "b-"},
			{Name: "C", Match: "c-"},
		},
	}
	if p.GroupOrder("A") != 0 {
		t.Errorf("expected 0, got %d", p.GroupOrder("A"))
	}
	if p.GroupOrder("B") != 1 {
		t.Errorf("expected 1, got %d", p.GroupOrder("B"))
	}
	if p.GroupOrder("C") != 2 {
		t.Errorf("expected 2, got %d", p.GroupOrder("C"))
	}
	if p.GroupOrder("unknown") != 9999 {
		t.Errorf("expected 9999 for unknown, got %d", p.GroupOrder("unknown"))
	}
}

// ── CacheKey ──────────────────────────────────────────────────────────────

func TestCacheKey_ChangesOnOwnerChange(t *testing.T) {
	c1 := Config{Profiles: []Profile{{Owner: "org-a", Prefixes: []string{"svc-"}}}}
	c2 := Config{Profiles: []Profile{{Owner: "org-b", Prefixes: []string{"svc-"}}}}
	if c1.CacheKey() == c2.CacheKey() {
		t.Error("cache keys should differ when owner changes")
	}
}

func TestCacheKey_ChangesOnPrefixChange(t *testing.T) {
	c1 := Config{Profiles: []Profile{{Owner: "org", Prefixes: []string{"a-"}}}}
	c2 := Config{Profiles: []Profile{{Owner: "org", Prefixes: []string{"b-"}}}}
	if c1.CacheKey() == c2.CacheKey() {
		t.Error("cache keys should differ when prefixes change")
	}
}

func TestCacheKey_StableAcrossCallsAndPrefixOrder(t *testing.T) {
	c := Config{Profiles: []Profile{{Owner: "org", Prefixes: []string{"b-", "a-"}}}}
	// Sorted prefixes → same key regardless of input order
	c2 := Config{Profiles: []Profile{{Owner: "org", Prefixes: []string{"a-", "b-"}}}}
	if c.CacheKey() != c2.CacheKey() {
		t.Error("cache keys should be stable regardless of prefix order")
	}
	// Called twice → same result
	if k1, k2 := c.CacheKey(), c.CacheKey(); k1 != k2 {
		t.Errorf("cache key is not deterministic: %q vs %q", k1, k2)
	}
}

func TestCacheKey_MultipleProfiles(t *testing.T) {
	c := Config{Profiles: []Profile{
		{Owner: "org-a", Prefixes: []string{"svc-"}},
		{Owner: "org-b", Prefixes: []string{"app-"}},
	}}
	// Adding a profile changes the key
	c2 := Config{Profiles: []Profile{
		{Owner: "org-a", Prefixes: []string{"svc-"}},
	}}
	if c.CacheKey() == c2.CacheKey() {
		t.Error("cache keys should differ with different profile counts")
	}
}

// ── Load — legacy migration ───────────────────────────────────────────────

func TestLoad_LegacyFlatConfig(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(p, []byte(`
base_path: /tmp/repos
org: my-org
prefixes:
  - svc-
  - infra-
`), 0o644)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)
	// Write to the right path
	xdgPath := filepath.Join(dir, "grove", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(xdgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(xdgPath, []byte(`
base_path: /tmp/repos
org: my-org
prefixes:
  - svc-
  - infra-
`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := Load()
	if len(cfg.Profiles) != 1 {
		t.Fatalf("expected 1 synthesised profile, got %d", len(cfg.Profiles))
	}
	if cfg.Profiles[0].Owner != "my-org" {
		t.Errorf("expected owner my-org, got %q", cfg.Profiles[0].Owner)
	}
	if len(cfg.Profiles[0].Prefixes) != 2 {
		t.Errorf("expected 2 prefixes, got %d", len(cfg.Profiles[0].Prefixes))
	}
}

func TestLoad_MultiProfileConfig(t *testing.T) {
	dir := t.TempDir()
	xdgPath := filepath.Join(dir, "grove", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(xdgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(xdgPath, []byte(`
profiles:
  - name: Personal
    owner: alice
    base_paths:
      - /home/alice/repos
    prefixes: []
  - name: Work
    owner: acme-corp
    base_paths:
      - /home/alice/work
    prefixes:
      - svc-
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)

	cfg := Load()
	if len(cfg.Profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(cfg.Profiles))
	}
	if cfg.Profiles[0].Name != "Personal" {
		t.Errorf("expected first profile named Personal, got %q", cfg.Profiles[0].Name)
	}
	if cfg.Profiles[1].Owner != "acme-corp" {
		t.Errorf("expected second profile owner acme-corp, got %q", cfg.Profiles[1].Owner)
	}
}

// ── ResolveGroups ─────────────────────────────────────────────────────────

func TestResolveGroups_ExplicitGroups(t *testing.T) {
	p := Profile{
		Groups: []Group{
			{Name: "A", Match: "a-"},
			{Name: "B", Match: "b-"},
		},
	}
	groups := p.ResolveGroups()
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].Name != "A" || groups[1].Name != "B" {
		t.Errorf("unexpected group names: %v", groups)
	}
}

func TestResolveGroups_DerivedFromPrefixes(t *testing.T) {
	p := Profile{Prefixes: []string{"svc-", "infra-"}}
	groups := p.ResolveGroups()
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	if groups[0].Name != "svc" {
		t.Errorf("expected svc, got %q", groups[0].Name)
	}
	if groups[1].Name != "infra" {
		t.Errorf("expected infra, got %q", groups[1].Name)
	}
}

func TestResolveGroups_EmptyProfile(t *testing.T) {
	p := Profile{}
	groups := p.ResolveGroups()
	if len(groups) != 0 {
		t.Errorf("expected 0 groups for empty profile, got %d", len(groups))
	}
}
