.PHONY: all test build clean lint fmt fmt-check vet tools sast verify-pins rubric cdk-synth cdk-deploy cdk-diff test-coverage-core coverage-core-report

# Local Go caches (sandbox-safe)
GOCACHE ?= $(CURDIR)/.gocache
GOMODCACHE ?= $(CURDIR)/.gomodcache
GOLANGCI_LINT_CACHE ?= $(CURDIR)/.golangci-lint-cache
export GOCACHE
export GOMODCACHE
export GOLANGCI_LINT_CACHE

# Hypergenium pinned tools (installed locally; never commit binaries).
HGM_TOOLS_BIN ?= $(CURDIR)/hgm-infra/.tools/bin
GOLANGCI_LINT_VERSION ?= v2.4.0
GOLANGCI_LINT_VERSION_NUM := $(subst v,,$(GOLANGCI_LINT_VERSION))
GOLANGCI_LINT ?= $(HGM_TOOLS_BIN)/golangci-lint

# Discover only packages that contain tests under ./pkg and selected roots.
# This avoids building unrelated packages (e.g., stacks) and excludes examples.
TEST_PKGS := $(shell find pkg -type f -name "*_test.go" -printf '%h\n' | sort -u | sed 's|^|./|' | grep -v '^\./pkg/cdk/integration$$')
TEST_PKGS += $(shell find benchmarks -type f -name "*_test.go" >/dev/null 2>&1 && echo ./benchmarks)

# Default target
all: test build

# Ensure local cache directories exist
cache-dirs:
	mkdir -p $(GOCACHE) $(GOMODCACHE) $(GOLANGCI_LINT_CACHE)

# Install pinned dev tools locally (deterministic; no "use whatever is installed").
$(GOLANGCI_LINT):
	mkdir -p $(HGM_TOOLS_BIN)
	@if [ -x "$(GOLANGCI_LINT)" ] && "$(GOLANGCI_LINT)" --version 2>/dev/null | grep -q "version $(GOLANGCI_LINT_VERSION_NUM)"; then \
		:; \
	else \
		GOBIN="$(HGM_TOOLS_BIN)" go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)"; \
	fi

tools: $(GOLANGCI_LINT)

# Run unit tests (exclude examples/ across the repo)
test: cache-dirs
	go test $(TEST_PKGS) -v

# Run tests with coverage (exclude examples/)
test-coverage: cache-dirs
	go test -covermode=atomic -coverprofile=coverage.out $(TEST_PKGS)
	go tool cover -func=coverage.out

# Run tests with coverage, then report "core" coverage excluding pkg/testing/**.
test-coverage-core: cache-dirs
	go test -covermode=atomic -coverprofile=coverage.out $(TEST_PKGS)
	bash ./scripts/coverage-core-report.sh coverage.out coverage.core.out

# Report "core" coverage from an existing coverage.out file.
coverage-core-report:
	bash ./scripts/coverage-core-report.sh coverage.out coverage.core.out

# Run tests with race detection (exclude examples/)
test-race: cache-dirs
	go test $(TEST_PKGS) -race -cover

# Build the project
build: cache-dirs
	go build ./...

# Clean build artifacts
clean:
	go clean
	rm -f coverage.out
	rm -rf cdk.out

# Format code
fmt:
	go fmt ./...

fmt-check:
	@files=$$(find . -type f -name '*.go' \
		-not -path './.git/*' \
		-not -path './.gocache/*' \
		-not -path './.gomodcache/*' \
		-not -path './.golangci-lint-cache/*' \
		-not -path './hgm-infra/*' \
		-print); \
	if [ -n "$$files" ]; then \
		out=$$(gofmt -l $$files); \
		if [ -n "$$out" ]; then \
			echo "gofmt changes required:"; \
			printf '%s\n' "$$out"; \
			exit 1; \
		fi; \
	fi

# Run go vet
vet: cache-dirs
	go vet ./...

# Run linter (requires golangci-lint)
lint: cache-dirs $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run --config .golangci.yml ./...

sast: cache-dirs $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run --config .golangci.yml --enable-only=gosec ./...

verify-pins:
	grep -q '^go 1.25$$' go.mod
	grep -q '1.25.x' .github/workflows/test.yml
	grep -q 'version: v2.4.0' .github/workflows/test.yml

rubric:
	bash ./hgm-infra/verifiers/hgm-verify-rubric.sh

# CDK targets
cdk-synth:
	cd pkg/cdk/examples/basic-api && cdk synth

cdk-deploy:
	cd pkg/cdk/examples/basic-api && cdk deploy

cdk-diff:
	cd pkg/cdk/examples/basic-api && cdk diff

cdk-destroy:
	cd pkg/cdk/examples/basic-api && cdk destroy

# Run benchmarks
bench:
	./benchmarks/run_benchmarks.sh

# Development helpers
dev: fmt vet test

# CI/CD helper
ci: fmt vet test-race lint
godoc-text:
	./scripts/generate-godoc.sh

.PHONY: doclint
doclint:
	# Pin revive version for reproducible CI
	go install github.com/mgechev/revive@v1.3.4
	# Use a minimal config to avoid failing CI on stylistic issues
	revive -config revive-docs.toml ./pkg/lift/... ./pkg/middleware/... ./pkg/lift/health/... ./pkg/lift/adapters/... ./pkg/observability/... || true
