#!/bin/sh

set -e

DIR=$( cd -P -- "$(dirname -- "$(command -v -- "$0")")" && pwd -P )

cd "${DIR}/.."

templ generate -path ./views --watch &

tailwindcss --config ./build/tailwind/tailwind.config.js \
	--input ./build/tailwind/input.css \
	--output assets/css/tailwind.css \
	--poll &

${GO_EXECUTABLE} run github.com/air-verse/air@latest \
	--build.cmd "make build" --build.bin "${TMP_DIR}/bin/${BINARY_NAME}" --build.delay "100" \
	--build.exclude_dir "views/tailwind" \
	--build.exclude_file "dashboard.yaml, assets/css/tailwind.css" \
	--build.include_ext "go, yaml" \
	--misc.clean_on_exit "true" &

wait
