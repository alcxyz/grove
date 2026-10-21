package config_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/alcxyz/grove/internal/config"
	"github.com/alcxyz/grove/internal/forge"
)

func TestResolvedInstanceUsesOnlyExplicitCredentials(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(map[bool]string{false: "anonymous", true: "explicit"}[explicit], func(t *testing.T) {
			seen := make(chan string, 8)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen <- r.Header.Get("Authorization")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte("[]"))
			}))
			defer server.Close()
			oldFile := filepath.Join(t.TempDir(), "old-token")
			newFile := filepath.Join(t.TempDir(), "new-token")
			for path, value := range map[string]string{oldFile: "synthetic-old", newFile: "synthetic-new"} {
				if err := os.WriteFile(path, []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			p := config.Profile{Owner: "example", Forge: "forgejo", InstanceURL: "https://old.example", TokenFile: oldFile, Social: config.Remote{InstanceURL: server.URL}}
			want := ""
			if explicit {
				p.Social.TokenFile = newFile
				want = "token synthetic-new"
			}
			r := p.SocialRemote("service-api", "")
			provider, err := forge.NewProvider(forge.ProviderConfig{Forge: r.EffectiveForge(), InstanceURL: r.InstanceURL, TokenFile: r.TokenFile, AuthMode: r.AuthMode})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = provider.ListIssues("example/service-api"); err != nil {
				t.Fatal(err)
			}
			select {
			case got := <-seen:
				if got != want {
					t.Fatalf("Authorization = %q, want %q", got, want)
				}
			default:
				t.Fatal("no request received")
			}
		})
	}
}
