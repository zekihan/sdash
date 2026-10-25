#!/bin/sh
# Install into this checkout only. Never trust an executable found on PATH.
set -eu
ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd -P)
cd "$ROOT"
TEMPL_VERSION="v0.3.1070"
TAILWINDCSS_VERSION="v4.3.3"
case $(uname -m) in
    x86_64) arch=x64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) echo "Unsupported architecture" >&2; exit 1 ;;
esac
case $(uname -s) in
    Darwin) os=macos ;;
    Linux) os=linux ;;
    *) echo "Unsupported OS" >&2; exit 1 ;;
esac
# Keep accepting the historical force flag; installs are now always verified.
case ${1-} in ''|-f) ;; *) echo "Usage: $0 [-f]" >&2; exit 1 ;; esac
mkdir -p build/bin
asset="tailwindcss-${os}-${arch}"
# Download to a non-executable temporary file. The verifier fails closed before
# the binary is installed, chmodded, or executed, including on cached installs.
download=$(mktemp "$ROOT/build/bin/.tailwindcss.XXXXXX")
trap 'rm -f "$download"' EXIT HUP INT TERM
curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 \
    "https://github.com/tailwindlabs/tailwindcss/releases/download/${TAILWINDCSS_VERSION}/${asset}" \
    --output "$download"
sh .github/scripts/verify_tailwind.sh "$asset" "$download"
chmod 755 "$download"
mv "$download" build/bin/tailwindcss
npm ci --ignore-scripts --prefix build/node
GOBIN="$ROOT/build/bin" go install "github.com/a-h/templ/cmd/templ@${TEMPL_VERSION}"
# Subsequent workflow steps resolve templ from this checkout too.
if [ -n "${GITHUB_PATH-}" ]; then
    printf '%s\n' "$ROOT/build/bin" >> "$GITHUB_PATH"
fi
