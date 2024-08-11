# Change these variables as necessary.
MAIN_PACKAGE_PATH := ./
BINARY_NAME := sdash
TMP_DIR := ./tmp
GO_EXECUTABLE := go

# ==================================================================================== #
# QUALITY CONTROL
# ==================================================================================== #

## tidy: format code and tidy modfile
.PHONY: tidy
tidy:
	templ fmt ./views
	${GO_EXECUTABLE} fmt ./...
	${GO_EXECUTABLE} mod tidy -v

## audit: run quality control checks
.PHONY: audit
audit:
	${GO_EXECUTABLE} mod verify
	${GO_EXECUTABLE} vet ./...
	${GO_EXECUTABLE} run honnef.co/go/tools/cmd/staticcheck@latest -checks=all,-ST1000,-U1000 ./...
	${GO_EXECUTABLE} run golang.org/x/vuln/cmd/govulncheck@latest ./...
	${GO_EXECUTABLE} test -race -buildvcs -vet=off ./...


# ==================================================================================== #
# DEVELOPMENT
# ==================================================================================== #

## test: run all tests
.PHONY: test
test:
	${GO_EXECUTABLE} test -v -race -buildvcs ./...

## test/cover: run all tests and display coverage
.PHONY: test/cover
test/cover:
	${GO_EXECUTABLE} test -v -race -buildvcs -coverprofile=${TMP_DIR}/coverage.out ./...
	${GO_EXECUTABLE} tool cover -html=${TMP_DIR}/coverage.out

## build: build the application
.PHONY: build
build:
	# Include additional build steps, like TypeScript, SCSS or Tailwind compilation here...
	templ generate -path ./views
	${GO_EXECUTABLE} build -o=${TMP_DIR}/bin/${BINARY_NAME} ${MAIN_PACKAGE_PATH}

## build: build the application
.PHONY: build_live
build_live:
	# Include additional build steps, like TypeScript, SCSS or Tailwind compilation here...
	${GO_EXECUTABLE} build -o=${TMP_DIR}/bin/${BINARY_NAME} ${MAIN_PACKAGE_PATH}

## run: run the  application
.PHONY: run
run: build
	${TMP_DIR}/bin/${BINARY_NAME}

## run: run the  application
.PHONY: templ_live
templ_live:
	templ generate -path ./views --watch

## run: run the  application
.PHONY: tailwind_live
tailwind_live:
	tailwindcss -o assets/css/tailwind.css --watch

## run_live: run the application with reloading on file changes
.PHONY: run_live
run_live:
	${GO_EXECUTABLE} run github.com/air-verse/air@latest \
		--build.cmd "make build_live" --build.bin "${TMP_DIR}/bin/${BINARY_NAME}" --build.delay "100" \
		--build.exclude_dir "" \
		--build.exclude_file "dashboard.yaml, assets/css/tailwind.css" \
		--build.include_ext "go, yaml" \
		--misc.clean_on_exit "true"

## run/live: run the application with reloading on file changes
.PHONY: run/live
run/live:
	make -j3 templ_live tailwind_live run_live
