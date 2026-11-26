# Build dependencies

Prerequisites: Go (see `go.mod`), Node 24+ with npm, GoReleaser and
golangci-lint for release/lint targets.

Run `sh .github/scripts/install_build_deps.sh` (or `make web`) before building.
The installer runs `npm ci --prefix web --ignore-scripts`, which installs
from `web/package-lock.json`, verifies tarball integrity, and disables all
install lifecycle scripts. The build runs without a dependency rebuild.

`make lint` checks dependency entries marked `hasInstallScript` in the
lockfile against the reviewed `allowScripts` in `web/package.json`. Each
entry requires an exact `name@version` approval set to `true`. The check
targets Linux on every host and exempts optional packages whose OS metadata
excludes Linux, such as `fsevents`. The PR and publication checks both run
this lint rule.

`make web` then builds the frontend into `web/dist`, which the Go binary
embeds. `web/dist` itself is not committed (only a `.gitkeep` placeholder, so
plain `go build` still compiles and serves a "run make web" notice for `/`).

For Docker packaging, run `make build/docker` (or `make build/release/docker`
for a release), then build `Dockerfile`. The image consumes the prepared
binaries from `dist/`; it does not install or run npm or Go tooling.

## Publication boundary

Nothing in the local build publishes artifacts. GoReleaser archive and Docker
image publication happen only through the tag-driven release workflow.
