.PHONY: all test build clean lint fmt vet cdk-synth cdk-deploy cdk-diff

# Default target
all: test build

# Run tests
test:
	go test ./pkg/... -v

# Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Run tests with race detection
test-race:
	go test ./... -race -cover

# Build the project
build:
	go build ./...

# Clean build artifacts
clean:
	go clean
	rm -f coverage.out
	rm -rf cdk.out

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Run linter (requires golangci-lint)
lint:
	golangci-lint run ./...

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