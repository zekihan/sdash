# Verified build dependencies

Run `sh .github/scripts/install_build_deps.sh` before `make tidy`, `make test`,
`make lint`, or a GoReleaser build. Node 24, npm, Go, curl, and a SHA-256 utility
(`sha256sum` or `shasum`) must be available. The installer writes only to the
checkout and the package managers' configured caches; it does not use sudo or
replace global executables. GoReleaser and golangci-lint remain prerequisites.

The installer always downloads Tailwind CSS v4.3.3 to a non-executable temporary
file, compares it with the platform's reviewed SHA-256 in
`build/tailwind/checksums.txt`, and installs it under `build/bin` only after a
match. An existing global or checkout-local Tailwind executable cannot bypass
verification. Unsupported platforms and missing, duplicate, or mismatching
checksums fail closed. The recorded hashes came from the upstream GitHub release
asset API on 2026-10-25; updating Tailwind requires reviewing and committing new
platform hashes together with the version change. Checksums are never fetched
at installation time.

`npm ci --ignore-scripts --prefix build/node` consumes the repository lockfile,
including integrity hashes and optional platform packages. Every formatting
entrypoint invokes `./build/node/node_modules/.bin/prettier` directly. Formatting
needs no network after installation and cannot fall back to registry acquisition.
Templ is installed at its fixed Go module version into `build/bin` as well.

For Docker packaging, run `make build/release/docker build/assets` after dependency
installation, then build `Dockerfile`. The CSS tooling previously installed during
image publication is now pinned in the same npm lockfile. The Dockerfile consumes
prepared binaries and CSS; it does not install or run npm tools.

## Publication boundary

Both publication workflows prepare artifacts in a job with contents-read
permissions, no publication secrets, and no persisted checkout credentials. The
release build generates its grouped changelog from git without a GitHub token.
A fresh dependent job downloads only artifacts from the same workflow run and
commit. The release publisher checks archive checksums before invoking `gh`;
the Docker publisher receives only prepared image inputs and packages them using
the existing digest-pinned base images. Dependency installation and formatting
do not occur in the credentialed jobs.

GitHub release creation uses the prepared `CHANGELOG.md` with the existing image-pull footer, verifies the tag already
exists, and marks hyphenated prerelease tags as prereleases. It fails if a release
already exists rather than replacing it; operators must review retries of an
already-published release. No real release or registry publication is part of
local validation. Release reruns, stable/prerelease publication, artifact transfer,
and multi-platform image publication need acceptance in GitHub with separate
authorization to publish.

## Local checks

- `python3 -m unittest discover -s scripts/tests -v` runs synthetic checksum,
  installer-failure, publication-boundary, and mocked release fixtures.
- `shellcheck .github/scripts/*.sh` and `actionlint` validate shell/workflow syntax.
- Run the repository's `make test`, `make lint`, and `make build` targets.
- `goreleaser release --snapshot --clean --parallelism 1` verifies release archive
  preparation without publishing. `make build/assets` exercises locked CSS tools.

On shared machines, set `GOMAXPROCS=2`, `GOFLAGS=-p=2`, and an appropriate
`GOMEMLIMIT`. Keep task-local Go caches in a hidden directory (for example
`.cache/go`), so recursive formatters do not scan downloaded modules.
