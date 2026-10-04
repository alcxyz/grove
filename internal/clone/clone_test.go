package clone

import (
	"testing"

	"github.com/alcxyz/grove/internal/config"
)

func TestCloneDestFor(t *testing.T) {
	profile := config.Profile{
		BasePaths: []string{"/src"},
		Groups: []config.Group{
			{Name: "nix", MatchPath: "/src/nix", BasePath: "/src/nix"},
			{Name: "svc", Match: "svc-", BasePath: "/src/services"},
			{Name: "rest", BasePath: "/src/misc"},
		},
	}
	for _, tc := range []struct {
		repo string
		want string
	}{
		{"svc-api", "/src/services"},
		{"dotfiles", "/src/misc"},
	} {
		if got := cloneDestFor(profile, tc.repo); got != tc.want {
			t.Errorf("cloneDestFor(%q) = %q, want %q", tc.repo, got, tc.want)
		}
	}

	profile.Groups = profile.Groups[:2]
	if got := cloneDestFor(profile, "dotfiles"); got != "/src" {
		t.Errorf("match_path-only group must not catch unmatched repos: got %q, want /src", got)
	}
}
