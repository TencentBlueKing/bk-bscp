# Makefile for gsekit-migration tool

.PHONY: build clean

BINARY_NAME=bk-bscp-gsekit-migration
BUILD_DIR=../../build/bk-bscp/bk-bscp-gsekit-migration

build:
	@echo "Building $(BINARY_NAME)..."
	go build -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Build completed: $(BUILD_DIR)/$(BINARY_NAME)"

clean:
	@echo "Cleaning..."
	rm -f $(BUILD_DIR)/$(BINARY_NAME)
	@echo "Clean completed"

.DEFAULT_GOAL := build
