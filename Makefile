.PHONY: all web-build build build-all clean test lint fmt deps run help \
        container container-gateway container-stop container-logs

# Build variables
BINARY_NAME=localagent
BUILD_DIR=build
CMD_DIR=cmd
MAIN_GO=$(CMD_DIR)/main.go

# Version
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Go variables
GO?=go
GOFLAGS?=-v

# Container variables
CONTAINER_ENGINE?=podman
CONTAINER_IMAGE?=localagent
CONTAINER_TAG?=$(VERSION)
CONTAINER_NAME?=localagent-gateway
CONFIG_FILE?=$(HOME)/.localagent/config.json
WORKSPACE_DIR?=$(CURDIR)/.localagent/workspace
TZ?=$(shell cat /etc/timezone 2>/dev/null || readlink /etc/localtime 2>/dev/null | sed 's|.*/zoneinfo/||' || echo UTC)
CA_CERT?=
ENV_PASS?=
comma := ,

# Runtime: krun on Linux, not available on macOS
ifeq ($(UNAME_S),Linux)
	CONTAINER_RUNTIME_FLAG=--runtime=krun
else
	CONTAINER_RUNTIME_FLAG=
endif

# OS detection
UNAME_S:=$(shell uname -s)
UNAME_M:=$(shell uname -m)

ifeq ($(UNAME_S),Linux)
	PLATFORM=linux
	ifeq ($(UNAME_M),x86_64)
		ARCH=amd64
	else ifeq ($(UNAME_M),aarch64)
		ARCH=arm64
	else
		ARCH=$(UNAME_M)
	endif
else ifeq ($(UNAME_S),Darwin)
	PLATFORM=darwin
	ifeq ($(UNAME_M),x86_64)
		ARCH=amd64
	else ifeq ($(UNAME_M),arm64)
		ARCH=arm64
	else
		ARCH=$(UNAME_M)
	endif
else
	PLATFORM=$(UNAME_S)
	ARCH=$(UNAME_M)
endif

BINARY_PATH=$(BUILD_DIR)/$(BINARY_NAME)-$(PLATFORM)-$(ARCH)

all: build

## web-build: Build the Svelte SPA frontend
web-build:
	@echo "Building web frontend..."
	@cd web && bun install --frozen-lockfile && bun run build
	@echo "Web frontend built"

## build: Build the localagent binary for current platform
build: web-build
	@echo "Building $(BINARY_NAME) for $(PLATFORM)/$(ARCH)..."
	@mkdir -p $(BUILD_DIR)
	@CGO_ENABLED=0 $(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_PATH) ./$(CMD_DIR)
	@ln -sf $(BINARY_NAME)-$(PLATFORM)-$(ARCH) $(BUILD_DIR)/$(BINARY_NAME)
	@echo "Build complete: $(BINARY_PATH)"

## build-all: Build for all platforms
build-all: web-build
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./$(CMD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 ./$(CMD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./$(CMD_DIR)
	@echo "All builds complete"

## clean: Remove build artifacts
clean:
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete"

## test: Run tests
test:
	@$(GO) test ./...

## lint: Run go vet and deadcode
lint:
	@$(GO) vet ./...
	@deadcode ./...

## fmt: Format Go code
fmt:
	@gofmt -w .

## deps: Update dependencies
deps:
	@$(GO) get -u ./...
	@$(GO) mod tidy

## run: Build and run localagent
run: build
	@$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

## container: Build the container image
container:
	@echo "Building container image $(CONTAINER_IMAGE):$(CONTAINER_TAG)..."
	@$(CONTAINER_ENGINE) build -t $(CONTAINER_IMAGE):$(CONTAINER_TAG) -f Containerfile .
	@$(CONTAINER_ENGINE) tag $(CONTAINER_IMAGE):$(CONTAINER_TAG) $(CONTAINER_IMAGE):latest
	@$(CONTAINER_ENGINE) image prune -f --filter "until=24h" >/dev/null 2>&1 || true
	@echo "Container image built: $(CONTAINER_IMAGE):$(CONTAINER_TAG)"

## container-gateway: Run gateway
container-gateway:
	@mkdir -p $(WORKSPACE_DIR)
	@echo "Starting localagent gateway..."
	@$(CONTAINER_ENGINE) run -d \
		--name $(CONTAINER_NAME) \
		$(CONTAINER_RUNTIME_FLAG) \
		--network=pasta \
		--restart=unless-stopped \
		-e TZ=$(TZ) \
		$(foreach v,$(ENV_PASS),-e $(v)) \
		$(if $(CA_CERT),-v $(CA_CERT):/usr/local/share/ca-certificates/custom-ca.crt:ro$(comma)Z) \
		-v $(CONFIG_FILE):/home/localagent/.localagent/config.json:ro,Z \
		-v $(WORKSPACE_DIR):/home/localagent/.localagent/workspace:Z \
		-p 18790:18790 \
		-p 18791:18791 \
		$(CONTAINER_IMAGE):latest gateway
	@echo "Gateway started: $(CONTAINER_NAME)"

## container-stop: Stop and remove the container
container-stop:
	@$(CONTAINER_ENGINE) stop $(CONTAINER_NAME) 2>/dev/null || true
	@$(CONTAINER_ENGINE) rm $(CONTAINER_NAME) 2>/dev/null || true
	@echo "Container stopped and removed"

## container-logs: Show container logs
container-logs:
	@$(CONTAINER_ENGINE) logs -f $(CONTAINER_NAME)

## help: Show this help message
help:
	@echo "localagent Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
	@echo ""
	@echo "Container Variables:"
	@echo "  CONTAINER_ENGINE   Container runtime (default: podman)"
	@echo "  CONTAINER_IMAGE    Image name (default: localagent)"
	@echo "  CONFIG_FILE        Config file path (default: ~/.localagent/config.json)"
	@echo "  WORKSPACE_DIR      Workspace directory (default: ./.localagent/workspace)"
	@echo ""
	@echo "Current Configuration:"
	@echo "  Platform: $(PLATFORM)/$(ARCH)"
	@echo "  Binary: $(BINARY_PATH)"
