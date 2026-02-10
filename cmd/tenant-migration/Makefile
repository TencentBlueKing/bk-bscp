# Makefile for tenant-migration tool

.PHONY: build build-dev clean

BINARY_NAME=bk-bscp-tenant-migration
BUILD_DIR=../../build/bk-bscp/bk-bscp-tenant-migration

# Production build - without testdata commands (for operations)
build:
	@echo "Building $(BINARY_NAME) (production)..."
	go build -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Build completed: $(BUILD_DIR)/$(BINARY_NAME)"

# Development build - with testdata commands (for testing)
build-dev:
	@echo "Building $(BINARY_NAME) (development with testdata)..."
	go build -tags testdata -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Build completed: $(BUILD_DIR)/$(BINARY_NAME)"

clean:
	@echo "Cleaning..."
	rm -f $(BUILD_DIR)/$(BINARY_NAME)
	@echo "Clean completed"

run:
	go run main.go

run-dev:
	go run -tags testdata main.go

test:
	go test -v ./...

test-dev:
	go test -tags testdata -v ./...

lint:
	golangci-lint run ./...
