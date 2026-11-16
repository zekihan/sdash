# Build dependencies

Prerequisites: Go (see `go.mod`), Node 24+ with npm, GoReleaser and
golangci-lint for release/lint targets.

Run `sh .github/scripts/install_build_deps.sh` (or `make web`) before building.
It runs `npm ci --prefix web`, which installs exactly what
`web/package-lock.json` pins and verifies every tarball against the lockfile
integrity hashes. The only approved package install script is esbuild's,
recorded under `allowScripts` in `web/package.json`; approving another one is
a reviewed change to that file.

`make web` then builds the frontend into `web/dist`, which the Go binary
embeds. `web/dist` itself is not committed (only a `.gitkeep` placeholder, so
plain `go build` still compiles and serves a "run make web" notice for `/`).

For Docker packaging, run `make build/docker` (or `make build/release/docker`
for a release), then build `Dockerfile`. The image consumes the prepared
binaries from `dist/`; it does not install or run npm or Go tooling.

## Publication boundary

Nothing in the local build publishes artifacts. GoReleaser archive and Docker
image publication happen only through the tag-driven release workflow.
