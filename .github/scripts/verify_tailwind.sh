#!/bin/sh
# Compare only against reviewed repository values, never a downloaded checksum.
set -eu
ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd -P)
if [ "$#" -ne 2 ]; then
    echo "Usage: $0 asset-name downloaded-file" >&2
    exit 1
fi
asset=$1
file=$2
expected=$(awk -v asset="$asset" '$2 == asset { print $1 }' "$ROOT/build/tailwind/checksums.txt")
if [ "${#expected}" -ne 64 ]; then
    echo "Missing or invalid Tailwind checksum for $asset" >&2
    exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$file" | cut -d ' ' -f 1)
else
    actual=$(shasum -a 256 "$file" | cut -d ' ' -f 1)
fi
if [ "$actual" != "$expected" ]; then
    echo "Tailwind checksum mismatch for $asset" >&2
    exit 1
fi
