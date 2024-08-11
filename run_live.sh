#!/bin/bash

templ generate -path ./views --watch &

tailwindcss -o assets/css/tailwind.css --watch &

${GO_EXECUTABLE} run github.com/air-verse/air@latest \
		--build.cmd "make build_live" --build.bin "${TMP_DIR}/bin/${BINARY_NAME}" --build.delay "100" \
		--build.exclude_dir "" \
		--build.exclude_file "dashboard.yaml, assets/css/tailwind.css" \
		--build.include_ext "go, yaml" \
		--misc.clean_on_exit "true" &

wait
