BINARY := withingy
BUILD_DIR := bin
BUILD_PATH := $(BUILD_DIR)/$(BINARY)
INSTALL_DIR := $(HOME)/.local/bin
GOFMT := gofmt

.PHONY: all build install clean test fmt

all: build

build:
	@echo "Building $(BINARY)..."
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_PATH) ./cmd/withingy

install: build
	@echo "Installing $(BINARY) to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR)
	cp $(BUILD_PATH) $(INSTALL_DIR)/$(BINARY)
	@echo "Installed $(INSTALL_DIR)/$(BINARY)"

clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)

test:
	go test ./...

fmt:
	$(GOFMT) -w $(shell go list -f '{{.Dir}}' ./...)
