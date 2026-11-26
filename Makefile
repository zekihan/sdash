# Change these variables as necessary.
MAIN_PACKAGE_PATH := $(shell pwd)
BINARY_NAME := $(shell basename $(MAIN_PACKAGE_PATH))
OS_INFO := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH_INFO := $(shell uname -m | sed 's/x86_64/amd64/')
TMP_DIR := ./tmp
GO_EXECUTABLE := go

## web: install frontend dependencies and build the web UI into web/dist
.PHONY: web
web:
	sh .github/scripts/install_build_deps.sh
	npm --prefix web run build

## web/dev: run the Vite dev server (proxies /api to the Go server on :33000)
.PHONY: web/dev
web/dev:
	npm --prefix web run dev

# build: build the binary (frontend included) for the local platform
.PHONY: build
build: web tidy build/local

.PHONY: run
run: build
	./dist/${BINARY_NAME}_${OS_INFO}_${ARCH_INFO}/${BINARY_NAME}

## dev: run the Go server with the checked-in dev config (build web first)
.PHONY: dev
dev: web
	${GO_EXECUTABLE} run .

## tidy: format code and tidy modfile
.PHONY: tidy
tidy:
	${GO_EXECUTABLE} fmt ./...
	${GO_EXECUTABLE} mod tidy -v
	@if [ -d web/node_modules ]; then \
		npm --prefix web run typecheck; \
	else \
		echo "skip: web typecheck (web/node_modules is missing; run 'make web' first)"; \
	fi

## lint: run linter
.PHONY: lint
lint: tidy
	node web/scripts/check-install-scripts.mjs
	golangci-lint fmt
	golangci-lint run --fix > /dev/null 2>&1 || true
	golangci-lint run

## audit: run quality control checks
.PHONY: audit
audit: tidy
	${GO_EXECUTABLE} mod verify
	${GO_EXECUTABLE} vet ./...
	${GO_EXECUTABLE} run honnef.co/go/tools/cmd/staticcheck@latest -checks=all,-ST1000,-U1000 ./...
	${GO_EXECUTABLE} run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
	./scripts/go_test.sh -buildvcs -vet=all ./...

## test: run all tests
.PHONY: test
test:
	npm --prefix web test
	./scripts/go_test.sh -v -buildvcs -vet=all ./...
	./scripts/go_test.sh -v -buildvcs -vet=all -race ./...

## test/cover: run all tests and display coverage
.PHONY: test/cover
test/cover:
	mkdir -p ${TMP_DIR}
	./scripts/go_test.sh -v -buildvcs -vet=all -coverprofile=${TMP_DIR}/coverage.out ./...
	${GO_EXECUTABLE} tool cover -html=${TMP_DIR}/coverage.out

# build/local: build the binary for local development
.PHONY: build/local
build/local:
	goreleaser build --clean --id all --snapshot --single-target

# build/all: build the binary for all platforms
.PHONY: build/all
build/all: web
	goreleaser build --clean --id all --snapshot

# build/docker: build the binaries the Docker image packages
.PHONY: build/docker
build/docker: web
	goreleaser build --clean --id docker --snapshot

# build/release: build the binary for release
.PHONY: build/release
build/release: build/release/all

# build/release/all: build the binary for release
.PHONY: build/release/all
build/release/all: web
	goreleaser build --clean --id all --skip=validate

# build/release/docker: build the docker binaries for release
.PHONY: build/release/docker
build/release/docker: web
	goreleaser build --clean --id docker --skip=validate

# release: prepare local release artifacts without publishing
.PHONY: release
release: web
	goreleaser release --clean --skip=publish,announce

# new_version: create a new version
.PHONY: new_version
new_version:
	./scripts/new_version.sh $(VERSION)
