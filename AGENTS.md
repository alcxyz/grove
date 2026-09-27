# Grove repository instructions

- GitHub `origin` is authoritative for branches, pull requests, CI, and releases. Forgejo is a continuity mirror;
  do not integrate the same change there. See [ADR-011](docs/adr/ADR-011-split-host-integration-authority.md).
- Develop from `dev` and target GitHub pull requests at `dev`. Protected `main` accepts only same-repository `dev`
  promotion pull requests with a new release version; see [CONTRIBUTING.md](CONTRIBUTING.md).
- Before lasting design changes, read the [ADR index](docs/adr/README.md) and relevant accepted decisions,
  especially the provider and remote-concern decisions for forge behavior.
- Keep code in its existing `cmd/grove` and `internal` packages; use [CONTRIBUTING.md](CONTRIBUTING.md) for the
  package map.
- For Go changes, run `gofmt`, `go test ./...`, and `go vet ./...`; CI also builds, lints, runs race tests, and
  checks the release snapshot and Nix build. See [.github/workflows/ci.yml](.github/workflows/ci.yml).
- `VERSION` drives releases. Follow the `dev` to `main` promotion and post-release version flow in
  [CONTRIBUTING.md](CONTRIBUTING.md); do not hand-create a parallel release on the mirror.
