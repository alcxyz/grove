{
  description = "grove — terminal UI for monitoring git forge repositories";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        sourceVersion = builtins.replaceStrings ["\n"] [""] (builtins.readFile ./VERSION);
        revision = self.rev or self.dirtyRev or "unknown";
        developmentVersion = import ./build-version.nix {
          version = sourceVersion;
          inherit revision;
        };
        releaseVersion = import ./build-version.nix {
          version = sourceVersion;
          inherit revision;
          release = true;
        };
        mkGrove = version: pkgs.buildGoModule {
          pname = "grove";
          inherit version;
          src = self;

          # Update when go.mod changes:
          #   nix build .# 2>&1 | grep "got:" | awk '{print $2}'
          vendorHash = "sha256-H3UWahntqhEIhYvaX9JyUe27lraVDeh5zUzE2tLs28o=";

          subPackages = [ "cmd/grove" ];
          ldflags = [ "-s" "-w" "-X main.version=${version}" ];

          meta = with pkgs.lib; {
            description = "Terminal UI for monitoring git forge repositories";
            homepage = "https://github.com/alcxyz/grove";
            license = licenses.mit;
            mainProgram = "grove";
          };
        };
      in {
        packages = (rec {
          grove = mkGrove developmentVersion;
          default = grove;
        }) // pkgs.lib.optionalAttrs
          (builtins.match "(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)\\.(0|[1-9][0-9]*)" sourceVersion != null
            && builtins.match "[0-9a-f]{7,64}" revision != null)
          { release = mkGrove releaseVersion; };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [ go gopls gotools goreleaser ];
        };
      }
    );
}
