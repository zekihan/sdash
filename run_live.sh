#!/bin/bash

templ generate -path ./views --watch &

tailwindcss --config ./views/tailwind/tailwind.config.js \
	-i ./views/tailwind/input.css \
	-o assets/css/tailwind.css --watch &

${GO_EXECUTABLE} run github.com/air-verse/air@latest \
	--build.cmd "make build" --build.bin "${TMP_DIR}/bin/${BINARY_NAME}" --build.delay "100" \
	--build.exclude_dir "views/tailwind" \
	--build.exclude_file "dashboard.yaml, assets/css/tailwind.css" \
	--build.include_ext "go, yaml" \
	--misc.clean_on_exit "true" &

wait
