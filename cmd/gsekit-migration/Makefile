# Makefile for gsekit-migration tool

.PHONY: build clean docker push-image

BINARY_NAME=bk-bscp-gsekit-migration
BUILD_DIR=../../build/bk-bscp/bk-bscp-gsekit-migration
PRO_DIR=../..
REPO ?= mirrors.tencent.com/bk-bcs-xiaolnwang/gsekit-migrate
TAG  ?= latest

build:
	@echo "Building $(BINARY_NAME)..."
	CGO_ENABLED=0 go build -trimpath -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Build completed: $(BUILD_DIR)/$(BINARY_NAME)"

docker: build
	@echo "Building Docker image..."
	docker build -t $(REPO):$(TAG) -f Dockerfile $(PRO_DIR)
	@echo "Docker image built: $(REPO):$(TAG)"

push-image: docker
	@echo "Pushing Docker image..."
	docker push $(REPO):$(TAG)
	@echo "Image pushed: $(REPO):$(TAG)"

clean:
	@echo "Cleaning..."
	rm -f $(BUILD_DIR)/$(BINARY_NAME)
	@echo "Clean completed"

.DEFAULT_GOAL := build
