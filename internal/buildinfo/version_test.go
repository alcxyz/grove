package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestResolve(t *testing.T) {
	revision := "ABCDEF0123456789ABCDEF0123456789ABCDEF01"
	clean := &debug.BuildInfo{Settings: []debug.BuildSetting{
		{Key: "vcs.revision", Value: revision},
		{Key: "vcs.modified", Value: "false"},
	}}
	dirty := &debug.BuildInfo{Settings: []debug.BuildSetting{
		{Key: "vcs.modified", Value: "true"},
		{Key: "vcs.revision", Value: revision},
	}}

	tests := []struct {
		name       string
		configured string
		info       *debug.BuildInfo
		want       string
	}{
		{name: "clean development build", configured: "dev", info: clean, want: "dev-abcdef012345"},
		{name: "dirty development build", configured: "dev", info: dirty, want: "dev-abcdef012345-dirty"},
		{name: "missing metadata", configured: "dev", want: "dev"},
		{name: "invalid revision", configured: "dev", info: &debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "not-a-commit"}}}, want: "dev"},
		{name: "release injection", configured: "0.9.2", info: dirty, want: "0.9.2"},
		{name: "Nix development injection", configured: "0.9.2-dev.abcdef012345.dirty", info: clean, want: "0.9.2-dev.abcdef012345.dirty"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolve(test.configured, test.info); got != test.want {
				t.Fatalf("resolve(%q) = %q, want %q", test.configured, got, test.want)
			}
		})
	}
}
