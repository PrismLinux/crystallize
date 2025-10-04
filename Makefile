# CrystalNetwork Studio
# <------------------->
# crystallize-cli Makefile
# Performance-optimized
# <------------------->

PROJECT_NAME := crystallize-cli
BINARY := $(PROJECT_NAME)
BIN_DIR := bin
GO := go
GOCACHE := $(shell $(GO) env GOCACHE)
export GOCACHE

BUILD_FLAGS := -trimpath
LDFLAGS := -ldflags="-s -w -buildid="

GO_SOURCES := $(shell find . -path ./vendor -prune -o -name '*.go' -print)

.DEFAULT_GOAL := build

# Main build target: build and compress
build: | $(BIN_DIR)
	@echo "Building optimized $(PROJECT_NAME)..."
	CGO_ENABLED=0 $(GO) build $(BUILD_FLAGS) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY) ./main.go
	@echo "Optimized build completed."

	@echo "Checking for UPX..."
	@which upx > /dev/null || (echo "Error: UPX not found. Please install UPX." && exit 1)
	@echo "Compressing binary with UPX..."
	upx --best --lzma $(BIN_DIR)/$(BINARY)
	@echo "UPX compression done: $(BIN_DIR)/$(BINARY)"

$(BIN_DIR):
	mkdir -p $@

clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BIN_DIR)
	$(GO) clean -cache -modcache
	@echo "Cleanup completed"

fmt:
	@echo "Formatting Go code..."
	$(GO) fmt ./...
	@echo "Code formatted"

install-autocompletion: build
	@echo "Generating Fish shell completion..."
	$(BIN_DIR)/$(BINARY) completion fish > $(BINARY).fish
	mkdir -p ~/.config/fish/completions
	cp $(BINARY).fish ~/.config/fish/completions/
	@echo "Fish shell completion installed."

help:
	@echo ""
	@echo "$(PROJECT_NAME) Makefile"
	@echo "Usage: make [target]"
	@echo ""
	@echo "  build                Build and compress binary with UPX"
	@echo "  clean                Clean build artifacts"
	@echo "  fmt                  Format Go code"
	@echo "  install-autocompletion Install Fish completion"
	@echo "  help                 Show this help"
	@echo ""

.PHONY: build clean fmt install-autocompletion help
