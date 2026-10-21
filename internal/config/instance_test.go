package config

import "testing"

func TestMergeRemoteInstanceCredentials(t *testing.T) {
	base := Remote{Owner: "example", Forge: "forgejo", InstanceURL: "https://old.example", TokenFile: "old-token", AuthMode: "tea"}
	for _, tt := range []struct {
		name        string
		override    Remote
		token, auth string
	}{
		{"empty", Remote{}, "old-token", "tea"},
		{"owner", Remote{Owner: "other"}, "old-token", "tea"},
		{"same instance", Remote{InstanceURL: "https://old.example"}, "old-token", "tea"},
		{"trailing slash", Remote{InstanceURL: "https://old.example/"}, "old-token", "tea"},
		{"new host", Remote{InstanceURL: "https://new.example"}, "", ""},
		{"new port", Remote{InstanceURL: "https://old.example:8443"}, "", ""},
		{"new scheme", Remote{InstanceURL: "http://old.example"}, "", ""},
		{"new path", Remote{InstanceURL: "https://old.example/other"}, "", ""},
		{"explicit token", Remote{InstanceURL: "https://new.example", TokenFile: "new-token"}, "new-token", ""},
		{"explicit auth", Remote{InstanceURL: "https://new.example", AuthMode: "tea"}, "", "tea"},
		{"explicit both", Remote{InstanceURL: "https://new.example", TokenFile: "new-token", AuthMode: "token"}, "new-token", "token"},
		{"forge", Remote{Forge: "github"}, "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeRemote(base, tt.override)
			if got.TokenFile != tt.token || got.AuthMode != tt.auth {
				t.Fatalf("credentials = %q, %q; want %q, %q", got.TokenFile, got.AuthMode, tt.token, tt.auth)
			}
		})
	}
}

func TestInstanceCredentialsAtEveryOverrideLayer(t *testing.T) {
	for _, layer := range []string{"profile social", "profile ci", "group code", "group social", "group ci", "repo code", "repo social", "repo ci", "path group"} {
		for _, explicit := range []bool{false, true} {
			t.Run(layer+map[bool]string{false: " inherited", true: " explicit"}[explicit], func(t *testing.T) {
				p := Profile{Owner: "example", Forge: "forgejo", InstanceURL: "https://old.example", TokenFile: "old-token", AuthMode: "tea"}
				ov := Remote{InstanceURL: "https://new.example"}
				want := ""
				if explicit {
					ov.TokenFile = "new-token"
					ov.AuthMode = "token"
					want = "new-token"
				}
				group := Group{Name: "services", Match: "service-"}
				repo := RepoOverride{Name: "service-api"}
				switch layer {
				case "profile social":
					p.Social = ov
				case "profile ci":
					p.CI = ov
				case "group code":
					group.Code = ov
				case "group social":
					group.Social = ov
				case "group ci":
					group.CI = ov
				case "repo code":
					repo.Code = ov
				case "repo social":
					repo.Social = ov
				case "repo ci":
					repo.CI = ov
				case "path group":
					group.Match = ""
					group.MatchPath = "/synthetic"
					group.Code = ov
				}
				p.Groups = []Group{group}
				p.Repos = []RepoOverride{repo}
				remotes := []Remote{p.CodeRemote("service-api", "/synthetic/service-api"), p.SocialRemote("service-api", "/synthetic/service-api"), p.CIRemote("service-api", "/synthetic/service-api")}
				c := Config{Profiles: []Profile{p}}
				remotes = append(remotes, c.AllRemotes()...)
				remotes = append(remotes, c.PathDependentRemotes()...)
				found := false
				for _, r := range remotes {
					if r.InstanceURL == ov.InstanceURL {
						found = true
						if r.TokenFile != want || (!explicit && r.AuthMode != "") {
							t.Fatalf("new instance credentials = %q, %q", r.TokenFile, r.AuthMode)
						}
					}
				}
				if !found {
					t.Fatal("new instance not resolved")
				}
			})
		}
	}
}

func TestInstanceCredentialResetDoesNotResurrectDefaults(t *testing.T) {
	p := Profile{Owner: "example", Forge: "forgejo", InstanceURL: "https://old.example", TokenFile: "old-token", AuthMode: "tea", Groups: []Group{{Match: "service-", Code: Remote{InstanceURL: "https://new.example", TokenFile: "new-token"}}}, Repos: []RepoOverride{{Name: "service-api", Code: Remote{InstanceURL: "https://old.example"}}}}
	if got := p.CodeRemote("service-api", ""); got.TokenFile != "" || got.AuthMode != "" {
		t.Fatalf("credentials restored: %+v", got)
	}
}

func TestGitHubIgnoresInstanceURLForCredentialDefaults(t *testing.T) {
	for _, forge := range []string{"", "github"} {
		base := Remote{Forge: forge, TokenFile: "synthetic-token", AuthMode: "gh"}
		got := mergeRemote(base, Remote{InstanceURL: "https://github.com"})
		if got.TokenFile != base.TokenFile || got.AuthMode != base.AuthMode {
			t.Fatalf("GitHub defaults changed: %+v", got)
		}
	}
}
