package forge

import "testing"

func TestForgejoCloneURLUsesConfiguredSSHHost(t *testing.T) {
	provider, err := NewForgejoProvider(ProviderConfig{
		Forge:       "forgejo",
		InstanceURL: "https://git.alc.xyz",
		CloneProto:  "ssh",
		SSHHost:     "ssh-git.alc.xyz",
	})
	if err != nil {
		t.Fatal(err)
	}

	got := provider.cloneURL("alcxyz", "grove")
	want := "git@ssh-git.alc.xyz:alcxyz/grove.git"
	if got != want {
		t.Fatalf("cloneURL() = %q, want %q", got, want)
	}
}

func TestForgejoCloneURLFallsBackToInstanceHost(t *testing.T) {
	provider, err := NewForgejoProvider(ProviderConfig{
		Forge:       "forgejo",
		InstanceURL: "https://git.example.com",
		CloneProto:  "ssh",
	})
	if err != nil {
		t.Fatal(err)
	}

	got := provider.cloneURL("team", "repo")
	want := "git@git.example.com:team/repo.git"
	if got != want {
		t.Fatalf("cloneURL() = %q, want %q", got, want)
	}
}
