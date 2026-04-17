{
  description = "grove — terminal UI for monitoring GitHub repositories";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        version = builtins.replaceStrings ["\n"] [""] (builtins.readFile ./VERSION);
      in {
        packages = rec {
          grove = pkgs.buildGoModule {
            pname = "grove";
            inherit version;
            src = self;

            # Update when go.mod changes:
            #   nix build .# 2>&1 | grep "got:" | awk '{print $2}'
            vendorHash = "sha256-Ea94DPGLBQhW3gILQlWQXVVSZzJxGVtr0vuzYZSSUsk=";

            subPackages = [ "cmd/grove" ];
            ldflags = [ "-s" "-w" "-X main.version=${version}" ];

            meta = with pkgs.lib; {
              description = "Terminal UI for monitoring GitHub repositories";
              homepage = "https://github.com/alcxyz/grove";
              license = licenses.mit;
              mainProgram = "grove";
            };
          };
          default = grove;
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [ go gopls gotools goreleaser ];
        };
      }
    );
}
