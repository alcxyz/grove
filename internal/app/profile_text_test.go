package app

import (
	"testing"

	"github.com/alcxyz/grove/internal/config"
)

func TestProfileHitTestUsesDisplayedText(t *testing.T) {
	for _, name := range []string{"Work   repos", "Work\x1b]0;hidden\arepos", "世界"} {
		t.Run(name, func(t *testing.T) {
			m := Model{cfg: config.Config{Profiles: []config.Profile{{Name: name}, {Name: "Home"}}}}
			width := map[string]int{"Work   repos": 12, "Work\x1b]0;hidden\arepos": 11, "世界": 6}[name]
			if got := m.profileAtX(width + 1); got != 1 {
				t.Fatalf("click on Home selects %d", got)
			}
			if got := m.profileAtX(width - 1); got != 0 {
				t.Fatalf("last column of first tab selects %d", got)
			}
		})
	}
}
