#!/bin/sh

set -e

DIR=$( cd -P -- "$(dirname -- "$(command -v -- "$0")")" && pwd -P )

cd "${DIR}/.."

templ generate -path ./views

tailwindcss --config ./build/tailwind/tailwind.config.js \
	--input ./build/tailwind/input.css \
	--output assets/css/tailwind.css
