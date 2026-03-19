.PHONY: lint lint-source docker-tools fmt fmt-check tidy-module tidy-module-check
.PHONY: unit unit-cover unit-race check-go-version build install clean release
.PHONY: itest itest-verbose help man sqlc
.PHONY: dashboard-build build-production

# Default target.
.DEFAULT_GOAL := build

# =========
# VARIABLES
# =========

PKG := github.com/lightninglabs/lnget
TOOLS_DIR := tools
DASHBOARD_DIR := dashboard

GOCC ?= go

GOIMPORTS_PKG := github.com/rinchsan/gosimports/cmd/gosimports

GO_BIN := $(GOPATH)/bin
GOIMPORTS_BIN := $(GO_BIN)/gosimports

# GO_VERSION is the Go version used for the release build, docker files, and
# GitHub Actions. This is the reference version for the project.
GO_VERSION := 1.25.3

GOBUILD := $(GOCC) build -v
GOINSTALL := $(GOCC) install -v
GOTEST := $(GOCC) test

GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*" -not -name "*pb.go" -not -name "*pb.gw.go" -not -name "*.pb.json.go")

RM := rm -f
MAKE := make
XARGS := xargs -L 1

COMMIT := $(shell git describe --tags --dirty 2>/dev/null || echo "unknown")

# Build tags.
DEV_TAGS := dev
LOG_TAGS := nolog
TEST_FLAGS :=
RELEASE_TAGS :=

# Build flags for debug builds.
DEV_GCFLAGS := -gcflags "all=-N -l"
DEV_LDFLAGS := -ldflags "-X $(PKG)/build.Commit=$(COMMIT)"

# Build flags for release builds.
RELEASE_LDFLAGS := -ldflags "-s -w -buildid= -X $(PKG)/build.Commit=$(COMMIT)"

ifneq ($(tags),)
DEV_TAGS += ${tags}
endif

# Logging tags - can be overridden with log= parameter.
# Examples: make unit log="stdlog trace"
# This enables stdout logging with trace level for debugging tests.
ifneq ($(log),)
LOG_TAGS := $(log)
endif

# Coverage settings.
COVER_PKG = $$($(GOCC) list -deps -tags="$(DEV_TAGS)" ./... | grep '$(PKG)')
COVER_FLAGS = -coverprofile=coverage.txt -covermode=atomic -coverpkg=$(PKG)/...

# Default: list all packages for testing.
GOLIST := $(GOCC) list -tags="$(DEV_TAGS)" ./...

# If specific package is being unit tested, construct the full name of the
# subpackage and narrow GOLIST to just that package.
ifneq ($(pkg),)
UNITPKG := $(PKG)/$(pkg)
GOLIST := $(GOCC) list -tags="$(DEV_TAGS)" $(UNITPKG)
COVER_PKG = $(PKG)/$(pkg)
COVER_FLAGS = -coverprofile=coverage.txt -covermode=atomic -coverpkg=$(PKG)/$(pkg)
endif

# If a specific unit test case is being targeted, construct test.run filter.
ifneq ($(case),)
TEST_FLAGS += -test.run=$(case)
endif

# If a timeout is specified, add it to test flags.
ifneq ($(timeout),)
TEST_FLAGS += -timeout=$(timeout)
endif

# Test commands.
UNIT := $(GOLIST) | $(XARGS) env $(GOTEST) -tags="$(DEV_TAGS) $(LOG_TAGS)" $(TEST_FLAGS)
UNIT_RACE := $(UNIT) -race
UNIT_COVER := $(GOTEST) $(COVER_FLAGS) -tags="$(DEV_TAGS) $(LOG_TAGS)" $(TEST_FLAGS) $(COVER_PKG)

# Linting uses a lot of memory, so keep it under control by limiting the number
# of workers if requested.
ifneq ($(workers),)
LINT_WORKERS_FLAG = --concurrency=$(workers)
endif

# Docker cache mounting strategy:
# - CI (GitHub Actions): Use bind mounts to host paths that GA caches persist.
# - Local: Use Docker named volumes (much faster on macOS/Windows).
ifdef CI
DOCKER_TOOLS = docker run \
  --rm \
  -v $${HOME}/.cache/go-build:/root/.cache/go-build \
  -v $${HOME}/.cache/golangci-lint:/root/.cache/golangci-lint \
  -v $${HOME}/go/pkg/mod:/go/pkg/mod \
  -e GOPATH=/go \
  -e LINT_WORKERS="$(LINT_WORKERS_FLAG)" \
  -v $$(pwd):/build lnget-tools
else
DOCKER_TOOLS = docker run \
  --rm \
  -v lnget-go-build-cache:/root/.cache/go-build \
  -v lnget-go-lint-cache:/root/.cache/golangci-lint \
  -v lnget-go-mod-cache:/go/pkg/mod \
  -e GOPATH=/go \
  -e LINT_WORKERS="$(LINT_WORKERS_FLAG)" \
  -v $$(pwd):/build lnget-tools
endif

GREEN := "\\033[0;32m"
NC := "\\033[0m"
define print
	@echo $(GREEN)$1$(NC)
endef

# Release build settings.
BUILD_SYSTEM := linux-amd64 linux-arm64 linux-armv7 darwin-amd64 darwin-arm64 windows-amd64

# By default we will build all systems. But with the 'sys' tag, a specific
# system can be specified.
ifneq ($(sys),)
BUILD_SYSTEM = $(sys)
endif

# ============
# DEPENDENCIES
# ============

$(GOIMPORTS_BIN):
	@$(call print, "Installing goimports.")
	cd $(TOOLS_DIR); $(GOCC) install -trimpath $(GOIMPORTS_PKG)

# ==============
# LINTING & CODE
# ==============

docker-tools:
	@$(call print, "Building tools docker image.")
	docker build -q -t lnget-tools $(TOOLS_DIR)

lint-source: docker-tools
	@$(call print, "Linting source.")
	$(DOCKER_TOOLS)

lint: check-go-version lint-source #? Run static code analysis

fmt: $(GOIMPORTS_BIN) #? Format code and fix imports
	@$(call print, "Fixing imports.")
	gosimports -w $(GOFILES_NOVENDOR)
	@$(call print, "Formatting source.")
	gofmt -l -w -s $(GOFILES_NOVENDOR)

fmt-check: fmt #? Verify code is formatted correctly
	@$(call print, "Checking fmt results.")
	if test -n "$$(git status --porcelain)"; then echo "code not formatted correctly, please run 'make fmt' again!"; git status; git diff; exit 1; fi

sqlc: #? Regenerate sqlc query code from SQL schemas
	@$(call print, "Generating sqlc.")
	sqlc generate

tidy-module: #? Run 'go mod tidy' for all modules
	@$(call print, "Running 'go mod tidy' for all modules")
	cd $(TOOLS_DIR) && go mod tidy
	go mod tidy

tidy-module-check: tidy-module #? Verify modules are up to date
	if test -n "$$(git status --porcelain)"; then echo "modules not updated, please run 'make tidy-module' again!"; git status; exit 1; fi

check-go-version:
	@$(call print, "Checking for target Go version (v$(GO_VERSION))")
	@./scripts/check-go-version.sh $(GO_VERSION)

# =======
# TESTING
# =======

unit: #? Run unit tests. Use pkg=<package> case=<test> timeout=<duration> log="stdlog trace"
	@$(call print, "Running unit tests.")
	$(UNIT)

unit-cover: #? Run unit tests with coverage
	@$(call print, "Running unit coverage tests.")
	$(UNIT_COVER)

unit-race: #? Run unit tests with race detector
	@$(call print, "Running unit race tests.")
	env CGO_ENABLED=1 GORACE="history_size=7 halt_on_errors=1" $(UNIT_RACE)

itest: #? Run integration tests
	@$(call print, "Running integration tests.")
	$(GOTEST) -tags "itest" -v ./itest/... -timeout 30m

itest-verbose: #? Run integration tests with verbose logging
	@$(call print, "Running integration tests with verbose logging.")
	$(GOTEST) -tags "itest" -v ./itest/... -timeout 30m -harness.logstdout

# ============
# BUILDING
# ============

build: #? Build debug binaries and place in project directory
	@$(call print, "Building debug binaries.")
	$(GOBUILD) -trimpath -tags="$(DEV_TAGS)" $(DEV_GCFLAGS) $(DEV_LDFLAGS) -o lnget ./cmd/lnget

install: dashboard-build #? Build and install binaries with embedded dashboard to GOPATH/bin
	@$(call print, "Installing binaries with embedded dashboard.")
	rm -rf api/dashboard_dist
	cp -r $(DASHBOARD_DIR)/out api/dashboard_dist
	$(GOINSTALL) -trimpath -tags="$(DEV_TAGS) dashboard" $(DEV_LDFLAGS) ./cmd/lnget

clean: #? Remove build artifacts
	@$(call print, "Cleaning build artifacts.")
	$(RM) ./lnget
	$(RM) -r ./bin
	$(RM) -r ./api/dashboard_dist

# ============
# RELEASE
# ============

release: #? Cross compile for all supported platforms
	@$(call print, "Cross compiling release binaries.")
	@mkdir -p ./bin
	@for sys in $(BUILD_SYSTEM); do \
		echo "Building for $$sys"; \
		export CGO_ENABLED=0 GOOS=$$(echo $$sys | cut -d- -f1) GOARCH=$$(echo $$sys | cut -d- -f2); \
		if [ "$$GOARCH" = "armv6" ]; then \
			export GOARCH=arm; export GOARM=6; \
		elif [ "$$GOARCH" = "armv7" ]; then \
			export GOARCH=arm; export GOARM=7; \
		fi; \
		$(GOBUILD) -trimpath $(RELEASE_LDFLAGS) -tags="$(RELEASE_TAGS)" -o ./bin/lnget-$$sys ./cmd/lnget; \
		echo; \
	done

# ==================
# DASHBOARD + BUNDLE
# ==================

dashboard-build: #? Build the Next.js dashboard for static export
	@$(call print, "Building dashboard.")
	cd $(DASHBOARD_DIR) && yarn install --frozen-lockfile && yarn build

build-production: dashboard-build #? Build production binary with embedded dashboard
	@$(call print, "Building production binary with embedded dashboard.")
	rm -rf api/dashboard_dist
	cp -r $(DASHBOARD_DIR)/out api/dashboard_dist
	$(GOBUILD) -trimpath -tags="$(RELEASE_TAGS) dashboard" $(RELEASE_LDFLAGS) -o lnget ./cmd/lnget
	@echo "Production build complete with embedded frontend."

# ==============
# DOCUMENTATION
# ==============

man: #? Generate man pages
	@$(call print, "Generating man pages.")
	$(GOCC) run docs/gen_man.go

# ============
# HELP
# ============

help: #? Show this help message
	@echo "Available make targets:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?#\? .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?#\\? "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Examples:"
	@echo "  make build"
	@echo "  make unit pkg=l402 case=TestStore timeout=5m"
	@echo "  make unit log=\"stdlog trace\" pkg=client"
	@echo "  make itest"
	@echo "  make release sys=darwin-arm64"
