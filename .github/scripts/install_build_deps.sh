#!/bin/sh
# Install frontend dependencies from the committed lockfile. The only approved
# package install script is esbuild's (see "allowScripts" in web/package.json);
# npm verifies every tarball against the lockfile integrity hashes.
set -eu
ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd -P)
cd "$ROOT"
case ${1-} in ''|-f) ;; *) echo "Usage: $0 [-f]" >&2; exit 1 ;; esac
npm ci --prefix web
