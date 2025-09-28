# crystallize-cli Makefile
# Performance-optimized

PROJECT_NAME := crystallize-cli
BINARY := $(PROJECT_NAME)
BIN_DIR := bin
GO := go
GOCACHE := $(shell $(GO) env GOCACHE)
export GOCACHE

# Performance-optimized build flags
BUILD_FLAGS := -trimpath
LDFLAGS := -ldflags="-s -w -buildid="

# Find all Go source files for proper dependency tracking
GO_SOURCES := $(shell find . -path ./vendor -prune -o -name '*.go' -print)

# Default target
.DEFAULT_GOAL := build

# Build the application with performance optimizations
build: $(BIN_DIR)/$(BINARY)

$(BIN_DIR)/$(BINARY): $(GO_SOURCES)
	@echo "Building optimized $(PROJECT_NAME)..."
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build $(BUILD_FLAGS) $(LDFLAGS) -o $@ ./main.go
	@echo "Optimized build completed: $@"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BIN_DIR)/*
	$(GO) clean -cache -modcache
	@echo "Cleanup completed"

# Format Go code efficiently
fmt:
	@echo "Formatting Go code..."
	$(GO) fmt ./...
	@echo "Code formatted"

# Install Fish shell completion
install-autocompletion: $(BIN_DIR)/$(BINARY)
	@echo "Generating Fish shell completion..."
	$(BIN_DIR)/$(BINARY) completion fish > $(BINARY).fish
	mkdir -p ~/.config/fish/completions
	cp $(BINARY).fish ~/.config/fish/completions/
	@echo "Fish shell completion installed to ~/.config/fish/completions/$(BINARY).fish"

# Help target
help:
	@echo ""
	@echo "$(PROJECT_NAME) Makefile"
	@echo "Usage: make [target]"
	@echo ""
	@echo "Build targets:"
	@echo "  build                Build the optimized application"
	@echo "  clean                Clean build artifacts and Go caches"
	@echo "  fmt                  Format Go code"
	@echo "  install-autocompletion Install Fish shell completion"
	@echo "  help                 Show this help message"
	@echo ""

.PHONY: build clean fmt install-autocompletion help
