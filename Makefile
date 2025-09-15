# Variables
BINARY_NAME=qnap-vm
VERSION := $(shell git describe --tags --always --dirty)
LDFLAGS=-ldflags "-X main.version=${VERSION}"
PLATFORMS=linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

# Default target
.PHONY: all
all: build

# Build the binary
.PHONY: build
build:
	go build ${LDFLAGS} -o bin/${BINARY_NAME} ./cmd/${BINARY_NAME}

# Build for all platforms
.PHONY: build-all
build-all:
	@for platform in ${PLATFORMS}; do \
		OS=$$(echo $$platform | cut -d'/' -f1); \
		ARCH=$$(echo $$platform | cut -d'/' -f2); \
		OUTPUT_NAME=${BINARY_NAME}-$$OS-$$ARCH; \
		if [ "$$OS" = "windows" ]; then OUTPUT_NAME=$$OUTPUT_NAME.exe; fi; \
		echo "Building $$OUTPUT_NAME..."; \
		GOOS=$$OS GOARCH=$$ARCH go build ${LDFLAGS} -o bin/$$OUTPUT_NAME ./cmd/${BINARY_NAME}; \
	done

# Run tests
.PHONY: test
test:
	go test -v -race -coverprofile=coverage.out ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage: test
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf bin/
	rm -f coverage.out coverage.html

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Run linter
.PHONY: vet
vet:
	go vet ./...

# Run quality checks
.PHONY: check
check: fmt vet test

# Install the binary
.PHONY: install
install: build
	cp bin/${BINARY_NAME} /usr/local/bin/

# Development setup
.PHONY: dev-setup
dev-setup:
	go mod download
	go mod tidy

# Run integration tests (requires QNAP hardware)
.PHONY: integration-test
integration-test:
ifndef NAS_HOST
	$(error NAS_HOST environment variable is required for integration tests)
endif
	cd tests/integration && go test -timeout 10m -integration .

# Run integration tests with coverage
.PHONY: integration-test-full
integration-test-full:
ifndef NAS_HOST
	$(error NAS_HOST environment variable is required for integration tests)
endif
	cd tests/integration && go test -timeout 10m -integration -coverprofile=../../integration_coverage.out .
	go tool cover -html=integration_coverage.out -o integration_coverage.html

# Run the binary
.PHONY: run
run: build
	./bin/${BINARY_NAME}

# Display help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build                Build the binary"
	@echo "  build-all           Build for all platforms"
	@echo "  test                Run unit tests"
	@echo "  test-coverage       Run tests with coverage report"
	@echo "  integration-test    Run integration tests against real QNAP hardware"
	@echo "  integration-test-full Run integration tests with coverage"
	@echo "  clean               Clean build artifacts"
	@echo "  fmt                 Format code"
	@echo "  vet                 Run go vet"
	@echo "  check               Run all quality checks"
	@echo "  install             Install the binary system-wide"
	@echo "  dev-setup           Set up development environment"
	@echo "  run                 Build and run the binary"
	@echo "  help                Display this help message"
	@echo ""
	@echo "Integration test environment variables:"
	@echo "  NAS_HOST            QNAP device hostname/IP (required)"
	@echo "  NAS_USER            SSH username (default: admin)"
	@echo "  NAS_SSH_KEY         Path to SSH private key (optional)"