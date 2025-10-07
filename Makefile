.PHONY: all build build-discovery build-tools clean test help

# Binary names
BINARY_NAME=restricted-proxy
BINARY_DISCOVERY=restricted-proxy-discovery
TOOL_LOGS_TO_CONFIG=logs-to-config

# Build flags
LDFLAGS_NORMAL=-ldflags "-X main.DiscoveryMode=false"
LDFLAGS_DISCOVERY=-ldflags "-X main.DiscoveryMode=true"

all: help

## build: Build the normal restricted proxy binary
build:
	@echo "Building normal restricted proxy..."
	go build $(LDFLAGS_NORMAL) -o $(BINARY_NAME) .
	@echo "Built: $(BINARY_NAME)"
	@sha256sum $(BINARY_NAME)

## build-discovery: Build the discovery mode proxy binary
build-discovery:
	@echo "Building discovery mode proxy..."
	go build $(LDFLAGS_DISCOVERY) -o $(BINARY_DISCOVERY) .
	@echo "Built: $(BINARY_DISCOVERY)"
	@sha256sum $(BINARY_DISCOVERY)

## build-both: Build both normal and discovery binaries
build-both: build build-discovery

## build-tools: Build the logs-to-config utility
build-tools:
	@echo "Building logs-to-config utility..."
	go build -o $(TOOL_LOGS_TO_CONFIG) ./cmd/logs-to-config
	@echo "Built: $(TOOL_LOGS_TO_CONFIG)"

## clean: Remove built binaries
clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME) $(BINARY_DISCOVERY) $(TOOL_LOGS_TO_CONFIG)

## test: Run tests
test:
	go test -v ./...

## help: Show this help message
help:
	@echo "Available targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
