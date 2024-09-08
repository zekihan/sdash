#!/bin/sh

set -e

DIR=$( cd -P -- "$(dirname -- "$(command -v -- "$0")")" && pwd -P )

cd "${DIR}/.."

if [ -z "${TMP_DIR}" ]; then
	TMP_DIR="./tmp"
fi

if [ -z "${BINARY_NAME}" ]; then
	BINARY_NAME="sdash"
fi

secrets="$(./scripts/inject_secrets.sh)"
retVal=$?
if [ $retVal -ne 0 ]; then
	echo "Failed to inject secrets"
	exit 1
fi
set -a
eval "${secrets}"
set +a

./scripts/build_deps.sh

./scripts/build.sh

./"${TMP_DIR}"/bin/"${BINARY_NAME}"

wait
