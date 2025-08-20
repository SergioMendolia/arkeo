# AutoTime Makefile
# Provides common development and build tasks

# Variables
BINARY_NAME=autotime
VERSION?=0.1.0
BUILD_DIR=build
DIST_DIR=dist

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

# Build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION)"
BUILD_FLAGS=-v $(LDFLAGS)

# Default target
.PHONY: all
all: clean fmt test build

# Build the binary
.PHONY: build
build:
	@echo "🔨 Building $(BINARY_NAME)..."
	$(GOBUILD) $(BUILD_FLAGS) -o $(BINARY_NAME) .
	@echo "✅ Build complete: $(BINARY_NAME)"

# Build for multiple platforms
.PHONY: build-all
build-all: clean
	@echo "🔨 Building for multiple platforms..."
	@mkdir -p $(DIST_DIR)

	# Linux AMD64
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 .

	# Linux ARM64
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(BUILD_FLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 .

	# macOS AMD64
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 .

	# macOS ARM64 (Apple Silicon)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(BUILD_FLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 .

	# Windows AMD64
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(BUILD_FLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe .

	@echo "✅ Cross-platform build complete. Binaries in $(DIST_DIR)/"

# Run tests
.PHONY: test
test:
	@echo "🧪 Running tests..."
	$(GOTEST) -v ./...

# Run tests with coverage
.PHONY: test-coverage
test-coverage:
	@echo "🧪 Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "📊 Coverage report generated: coverage.html"

# Format code
.PHONY: fmt
fmt:
	@echo "🎨 Formatting code..."
	$(GOFMT) ./...

# Lint code
.PHONY: lint
lint:
	@echo "🔍 Linting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "⚠️  golangci-lint not found. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Vet code
.PHONY: vet
vet:
	@echo "🔍 Vetting code..."
	$(GOCMD) vet ./...

# Run the application
.PHONY: run
run: build
	@echo "🚀 Running $(BINARY_NAME)..."
	./$(BINARY_NAME) timeline

# Run the demo
.PHONY: demo
demo: build
	@echo "🎭 Running demo..."
	./demo.sh

# Clean build artifacts
.PHONY: clean
clean:
	@echo "🧹 Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)
	rm -f coverage.out coverage.html

# Tidy dependencies
.PHONY: tidy
tidy:
	@echo "📦 Tidying dependencies..."
	$(GOMOD) tidy

# Download dependencies
.PHONY: deps
deps:
	@echo "📦 Downloading dependencies..."
	$(GOMOD) download

# Install the binary globally
.PHONY: install
install: build
	@echo "📥 Installing $(BINARY_NAME) globally..."
	cp $(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@echo "✅ $(BINARY_NAME) installed to /usr/local/bin/"

# Uninstall the binary
.PHONY: uninstall
uninstall:
	@echo "🗑️  Uninstalling $(BINARY_NAME)..."
	rm -f /usr/local/bin/$(BINARY_NAME)
	@echo "✅ $(BINARY_NAME) uninstalled"

# Development setup
.PHONY: setup
setup:
	@echo "⚙️  Setting up development environment..."
	$(GOMOD) download
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "📥 Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	@echo "✅ Development environment ready"

# Check if everything is ready for release
.PHONY: check
check: fmt vet lint test
	@echo "✅ All checks passed!"

# Create a release package
.PHONY: release
release: check build-all
	@echo "📦 Creating release package..."
	@mkdir -p $(DIST_DIR)

	# Create archives for each platform
	cd $(DIST_DIR) && tar -czf $(BINARY_NAME)-linux-amd64-$(VERSION).tar.gz $(BINARY_NAME)-linux-amd64
	cd $(DIST_DIR) && tar -czf $(BINARY_NAME)-linux-arm64-$(VERSION).tar.gz $(BINARY_NAME)-linux-arm64
	cd $(DIST_DIR) && tar -czf $(BINARY_NAME)-darwin-amd64-$(VERSION).tar.gz $(BINARY_NAME)-darwin-amd64
	cd $(DIST_DIR) && tar -czf $(BINARY_NAME)-darwin-arm64-$(VERSION).tar.gz $(BINARY_NAME)-darwin-arm64
	cd $(DIST_DIR) && zip $(BINARY_NAME)-windows-amd64-$(VERSION).zip $(BINARY_NAME)-windows-amd64.exe

	# Copy documentation
	cp README.md $(DIST_DIR)/
	cp INSTALL.md $(DIST_DIR)/
	cp config.example.yaml $(DIST_DIR)/

	@echo "🎉 Release package created in $(DIST_DIR)/"

# Show help
.PHONY: help
help:
	@echo "AutoTime Makefile Commands:"
	@echo ""
	@echo "Building:"
	@echo "  build       - Build binary for current platform"
	@echo "  build-all   - Build for all supported platforms"
	@echo "  install     - Install binary globally"
	@echo "  uninstall   - Remove globally installed binary"
	@echo ""
	@echo "Development:"
	@echo "  setup       - Set up development environment"
	@echo "  run         - Build and run timeline for today"
	@echo "  demo        - Run the demo script"
	@echo "  fmt         - Format code"
	@echo "  vet         - Vet code"
	@echo "  lint        - Lint code (requires golangci-lint)"
	@echo "  test        - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  check       - Run all checks (fmt, vet, lint, test)"
	@echo ""
	@echo "Dependencies:"
	@echo "  deps        - Download dependencies"
	@echo "  tidy        - Tidy dependencies"
	@echo ""
	@echo "Maintenance:"
	@echo "  clean       - Clean build artifacts"
	@echo "  release     - Create release package"
	@echo "  help        - Show this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make build               # Build for current platform"
	@echo "  make test                # Run tests"
	@echo "  make VERSION=1.0.0 release # Create v1.0.0 release"
