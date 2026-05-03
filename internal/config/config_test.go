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
		if got := p.GroupFor(c.repo, ""); got != c.want {
			t.Errorf("GroupFor(%q) = %q, want %q", c.repo, got, c.want)
		}
	}
}

func TestGroupFor_DerivedFromPrefixes(t *testing.T) {
	p := Profile{
		Prefixes: []string{"service-", "platform-"},
	}
	// Derived groups use the prefix minus trailing "-" as the name.
	if got := p.GroupFor("service-api", ""); got != "service" {
		t.Errorf("GroupFor(service-api) = %q, want service", got)
	}
	if got := p.GroupFor("platform-core", ""); got != "platform" {
		t.Errorf("GroupFor(platform-core) = %q, want platform", got)
	}
	if got := p.GroupFor("unknown", ""); got != "other" {
		t.Errorf("GroupFor(unknown) = %q, want other", got)
	}
}

func TestGroupFor_EmptyPrefixes(t *testing.T) {
	p := Profile{Prefixes: []string{}}
	// Empty prefixes + no groups → everything is "other"
	if got := p.GroupFor("anything", ""); got != "other" {
		t.Errorf("GroupFor(anything) = %q, want other", got)
	}
}

func TestGroupFor_MatchPath(t *testing.T) {
	p := Profile{
		Groups: []Group{
			{Name: "nix", MatchPath: "/home/user/nix"},
			{Name: "gitops", MatchPath: "/home/user/gitops"},
			{Name: "pages", Match: "github.io"},
		},
	}
	cases := []struct {
		repo string
		path string
		want string
	}{
		{"nix-config", "/home/user/nix/nix-config", "nix"},
		{"nix-secrets", "/home/user/nix/nix-secrets", "nix"},
		{"leantime-tidy", "/home/user/gitops/leantime-tidy", "gitops"},
		{"alcxyz.github.io", "/home/user/dev/alcxyz.github.io", "pages"},
		{"grove", "/home/user/dev/grove", "other"},
	}
	for _, c := range cases {
		if got := p.GroupFor(c.repo, c.path); got != c.want {
			t.Errorf("GroupFor(%q, %q) = %q, want %q", c.repo, c.path, got, c.want)
		}
	}
}

func TestGroupFor_MatchPathAndMatchMixed(t *testing.T) {
	// A group with both match and match_path — either one can trigger.
	p := Profile{
		Groups: []Group{
			{Name: "infra", Match: "infra-", MatchPath: "/home/user/infra"},
		},
	}
	// Matches via path
	if got := p.GroupFor("terraform", "/home/user/infra/terraform"); got != "infra" {
		t.Errorf("expected infra via path, got %q", got)
	}
	// Matches via name
	if got := p.GroupFor("infra-vpc", "/home/user/other/infra-vpc"); got != "infra" {
		t.Errorf("expected infra via name, got %q", got)
	}
	// No match
	if got := p.GroupFor("app", "/home/user/other/app"); got != "other" {
		t.Errorf("expected other, got %q", got)
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

func TestCacheKey_ChangesOnRemoteConcernChange(t *testing.T) {
	c1 := Config{Profiles: []Profile{{
		Name:  "test",
		Owner: "alcxyz",
		Social: Remote{
			Owner: "alcxyz",
			Forge: "github",
		},
	}}}
	c2 := Config{Profiles: []Profile{{
		Name:  "test",
		Owner: "alcxyz",
		Social: Remote{
			Owner: "alcxyz-alt",
			Forge: "github",
		},
	}}}
	if c1.CacheKey() == c2.CacheKey() {
		t.Error("cache keys should differ when social remote changes")
	}
}

func TestCacheKey_ChangesOnRemoteRepoNameChange(t *testing.T) {
	c1 := Config{Profiles: []Profile{{
		Name:  "test",
		Owner: "alcxyz",
		Repos: []RepoOverride{{
			Name: "annaetattoo",
			Code: Remote{
				Repo: "annaetattoo.github.io",
			},
		}},
	}}}
	c2 := Config{Profiles: []Profile{{
		Name:  "test",
		Owner: "alcxyz",
		Repos: []RepoOverride{{
			Name: "annaetattoo",
			Code: Remote{
				Repo: "annaetattoo-pages",
			},
		}},
	}}}
	if c1.CacheKey() == c2.CacheKey() {
		t.Error("cache keys should differ when a remote repo name override changes")
	}
}

func TestProfileRemoteResolution(t *testing.T) {
	p := Profile{
		Name:        "alcxyz",
		Owner:       "alcxyz",
		Forge:       "forgejo",
		InstanceURL: "https://git.alc.xyz",
		TokenFile:   "/tmp/forgejo-token",
		CloneProto:  "ssh",
		SSHHost:     "ssh-git.alc.xyz",
		Social: Remote{
			Owner: "alcxyz",
			Forge: "github",
		},
		CI: Remote{
			Owner: "alcxyz-ci",
			Forge: "github",
		},
		Repos: []RepoOverride{{
			Name: "hedgedoc",
			Social: Remote{
				Owner: "alcxyz-legacy",
			},
			CI: Remote{
				Owner: "alcxyz-legacy",
			},
		}},
	}

	code := p.CodeRemote("hedgedoc", "")
	if code.Owner != "alcxyz" || code.EffectiveForge() != "forgejo" || code.InstanceURL != "https://git.alc.xyz" || code.SSHHost != "ssh-git.alc.xyz" {
		t.Fatalf("unexpected code remote: %+v", code)
	}

	social := p.SocialRemote("grove", "")
	if social.Owner != "alcxyz" || social.EffectiveForge() != "github" {
		t.Fatalf("unexpected default social remote: %+v", social)
	}

	ci := p.CIRemote("grove", "")
	if ci.Owner != "alcxyz-ci" || ci.EffectiveForge() != "github" {
		t.Fatalf("unexpected default ci remote: %+v", ci)
	}

	overrideSocial := p.SocialRemote("hedgedoc", "")
	if overrideSocial.Owner != "alcxyz-legacy" || overrideSocial.EffectiveForge() != "github" {
		t.Fatalf("unexpected override social remote: %+v", overrideSocial)
	}

	overrideCI := p.CIRemote("hedgedoc", "")
	if overrideCI.Owner != "alcxyz-legacy" || overrideCI.EffectiveForge() != "github" {
		t.Fatalf("unexpected override ci remote: %+v", overrideCI)
	}
}

func TestProfileRemoteRepoNameOverride(t *testing.T) {
	p := Profile{
		Name:  "alcxyz",
		Owner: "alcxyz",
		Repos: []RepoOverride{{
			Name: "annaetattoo",
			Code: Remote{
				Repo: "annaetattoo.github.io",
			},
			Social: Remote{
				Owner: "alcxyz",
				Forge: "github",
				Repo:  "annaetattoo.github.io",
			},
		}},
	}

	code := p.CodeRemote("annaetattoo", "")
	if code.RepoName("annaetattoo") != "annaetattoo.github.io" {
		t.Fatalf("unexpected code repo name: %q", code.RepoName("annaetattoo"))
	}
	if code.FullName("annaetattoo") != "alcxyz/annaetattoo.github.io" {
		t.Fatalf("unexpected code full name: %q", code.FullName("annaetattoo"))
	}

	social := p.SocialRemote("annaetattoo", "")
	if social.FullName("annaetattoo") != "alcxyz/annaetattoo.github.io" {
		t.Fatalf("unexpected social full name: %q", social.FullName("annaetattoo"))
	}

	defaultRemote := p.CodeRemote("grove", "")
	if defaultRemote.RepoName("grove") != "grove" {
		t.Fatalf("unexpected default repo name: %q", defaultRemote.RepoName("grove"))
	}
}

func TestProfileGroupRemoteResolution(t *testing.T) {
	p := Profile{
		Name:        "alcxyz",
		Owner:       "alcxyz",
		Forge:       "forgejo",
		InstanceURL: "https://git.alc.xyz",
		Groups: []Group{
			{
				Name:      "forks",
				MatchPath: "/home/user/src/forks",
				Social: Remote{
					Owner: "alcxyz",
					Forge: "github",
				},
				CI: Remote{
					Owner: "alcxyz",
					Forge: "github",
				},
			},
		},
	}

	social := p.SocialRemote("frappe_docker", "/home/user/src/forks/frappe_docker")
	if social.Owner != "alcxyz" || social.EffectiveForge() != "github" {
		t.Fatalf("unexpected group social remote: %+v", social)
	}

	ci := p.CIRemote("frappe_docker", "/home/user/src/forks/frappe_docker")
	if ci.Owner != "alcxyz" || ci.EffectiveForge() != "github" {
		t.Fatalf("unexpected group ci remote: %+v", ci)
	}

	code := p.CodeRemote("frappe_docker", "/home/user/src/forks/frappe_docker")
	if code.Owner != "alcxyz" || code.EffectiveForge() != "forgejo" {
		t.Fatalf("unexpected group code remote: %+v", code)
	}
}

func TestAllRemotesIncludesGroupOverrides(t *testing.T) {
	c := Config{Profiles: []Profile{{
		Name:        "alcxyz",
		Owner:       "alcxyz",
		Forge:       "forgejo",
		InstanceURL: "https://git.alc.xyz",
		TokenFile:   "/tmp/forgejo-token",
		Groups: []Group{{
			Name:      "forks",
			MatchPath: "/home/user/src/forks",
			Social: Remote{
				Owner: "alcxyz",
				Forge: "github",
			},
			CI: Remote{
				Owner: "alcxyz",
				Forge: "github",
			},
		}},
	}}}

	seen := map[string]Remote{}
	for _, r := range c.AllRemotes() {
		seen[r.Key()] = r
	}

	groupSocial := mergeRemote(mergeRemote(c.Profiles[0].codeDefaults(), c.Profiles[0].Social), c.Profiles[0].Groups[0].Social)
	if _, ok := seen[groupSocial.Key()]; !ok {
		t.Fatalf("group social remote was not included in AllRemotes: %+v", groupSocial)
	}

	groupCI := mergeRemote(mergeRemote(c.Profiles[0].codeDefaults(), c.Profiles[0].CI), c.Profiles[0].Groups[0].CI)
	if _, ok := seen[groupCI.Key()]; !ok {
		t.Fatalf("group ci remote was not included in AllRemotes: %+v", groupCI)
	}
}

func TestMergeRemoteClearsForgeSpecificFieldsWhenForgeChanges(t *testing.T) {
	base := Remote{
		Owner:       "alcxyz",
		Forge:       "forgejo",
		InstanceURL: "https://git.alc.xyz",
		TokenFile:   "/tmp/forgejo-token",
		CloneProto:  "ssh",
		SSHHost:     "ssh-git.alc.xyz",
	}

	got := mergeRemote(base, Remote{Forge: "github"})
	if got.EffectiveForge() != "github" {
		t.Fatalf("expected github remote, got %+v", got)
	}
	if got.InstanceURL != "" || got.TokenFile != "" || got.CloneProto != "" || got.SSHHost != "" {
		t.Fatalf("forge-specific fields should be cleared when forge changes: %+v", got)
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
