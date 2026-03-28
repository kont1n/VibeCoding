# =============================================================================
# Face Grouper - Makefile
# =============================================================================
# Common Docker and development operations
# =============================================================================

.PHONY: help build test run clean push scan benchmark

# Variables
IMAGE_NAME := face-grouper
REGISTRY := ghcr.io/kont1n
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
ONNXRUNTIME_VERSION := 1.23.0

# Colors for output
BLUE := \033[0;34m
GREEN := \033[0;32m
YELLOW := \033[1;33m
NC := \033[0m # No Color

# Default target
help:
	@echo "$(BLUE)Face Grouper - Available Commands$(NC)"
	@echo "===================================="
	@echo ""
	@echo "$(GREEN)Build:$(NC)"
	@echo "  make build          - Build all Docker images"
	@echo "  make build-cpu      - Build CPU image"
	@echo "  make build-gpu      - Build NVIDIA GPU image"
	@echo "  make build-rocm     - Build AMD ROCm image"
	@echo ""
	@echo "$(GREEN)Run:$(NC)"
	@echo "  make run            - Run CPU container"
	@echo "  make run-gpu        - Run NVIDIA GPU container"
	@echo "  make run-rocm       - Run AMD ROCm container"
	@echo "  make run-compose    - Run with docker-compose"
	@echo ""
	@echo "$(GREEN)Test:$(NC)"
	@echo "  make test           - Run all tests"
	@echo "  make test-go        - Run Go unit tests"
	@echo "  make test-docker    - Test Docker images"
	@echo ""
	@echo "$(GREEN)Registry:$(NC)"
	@echo "  make push           - Push all images to registry"
	@echo "  make push-cpu       - Push CPU image"
	@echo "  make push-gpu       - Push GPU image"
	@echo "  make push-rocm      - Push ROCm image"
	@echo ""
	@echo "$(GREEN)Security:$(NC)"
	@echo "  make scan           - Scan all images for vulnerabilities"
	@echo "  make scan-cpu       - Scan CPU image"
	@echo "  make scan-gpu       - Scan GPU image"
	@echo "  make scan-rocm      - Scan ROCm image"
	@echo ""
	@echo "$(GREEN)Benchmark:$(NC)"
	@echo "  make benchmark      - Run CPU vs GPU benchmark"
	@echo ""
	@echo "$(GREEN)Cleanup:$(NC)"
	@echo "  make clean          - Remove all containers and images"
	@echo "  make clean-volumes  - Remove output volumes"
	@echo ""

# =============================================================================
# Build
# =============================================================================

build: build-cpu build-gpu build-rocm
	@echo "$(GREEN)✓ All images built successfully$(NC)"

build-cpu:
	@echo "$(BLUE)Building CPU image...$(NC)"
	docker build \
		-t $(IMAGE_NAME):cpu \
		-t $(IMAGE_NAME):cpu-$(VERSION) \
		-t $(REGISTRY)/$(IMAGE_NAME):cpu \
		-t $(REGISTRY)/$(IMAGE_NAME):cpu-$(VERSION) \
		-f Dockerfile \
		--build-arg ONNXRUNTIME_VERSION=$(ONNXRUNTIME_VERSION) \
		.

build-gpu:
	@echo "$(BLUE)Building NVIDIA GPU image...$(NC)"
	docker build \
		-t $(IMAGE_NAME):gpu \
		-t $(IMAGE_NAME):gpu-$(VERSION) \
		-t $(REGISTRY)/$(IMAGE_NAME):gpu \
		-t $(REGISTRY)/$(IMAGE_NAME):gpu-$(VERSION) \
		-f Dockerfile.nvidia \
		--build-arg ONNXRUNTIME_VERSION=$(ONNXRUNTIME_VERSION) \
		.

build-rocm:
	@echo "$(BLUE)Building AMD ROCm image...$(NC)"
	docker build \
		-t $(IMAGE_NAME):rocm \
		-t $(IMAGE_NAME):rocm-$(VERSION) \
		-t $(REGISTRY)/$(IMAGE_NAME):rocm \
		-t $(REGISTRY)/$(IMAGE_NAME):rocm-$(VERSION) \
		-f Dockerfile.rocm \
		--build-arg ONNXRUNTIME_VERSION=$(ONNXRUNTIME_VERSION) \
		.

# =============================================================================
# Run
# =============================================================================

run:
	@echo "$(BLUE)Running CPU container...$(NC)"
	docker run -d --rm \
		--name face-grouper-cpu \
		-v $(PWD)/dataset:/app/dataset:ro \
		-v $(PWD)/output:/app/output \
		-v $(PWD)/models:/app/models:ro \
		-p 8080:8080 \
		-e LOG_LEVEL=info \
		$(IMAGE_NAME):cpu

run-gpu:
	@echo "$(BLUE)Running NVIDIA GPU container...$(NC)"
	docker run -d --rm \
		--name face-grouper-gpu \
		--gpus all \
		-v $(PWD)/dataset:/app/dataset:ro \
		-v $(PWD)/output:/app/output \
		-v $(PWD)/models:/app/models:ro \
		-p 8081:8080 \
		-e GPU_ENABLED=1 \
		-e GPU_DEVICE_ID=0 \
		$(IMAGE_NAME):gpu

run-rocm:
	@echo "$(BLUE)Running AMD ROCm container...$(NC)"
	docker run -d --rm \
		--name face-grouper-rocm \
		--device /dev/kfd \
		--device /dev/dri \
		--group-add video \
		--group-add render \
		-v $(PWD)/dataset:/app/dataset:ro \
		-v $(PWD)/output:/app/output \
		-v $(PWD)/models:/app/models:ro \
		-p 8082:8080 \
		-e GPU_ENABLED=1 \
		-e PROVIDER_PRIORITY=rocm \
		$(IMAGE_NAME):rocm

run-compose:
	@echo "$(BLUE)Running with docker-compose...$(NC)"
	docker-compose up -d

stop:
	@echo "$(YELLOW)Stopping containers...$(NC)"
	docker-compose down || true
	docker stop face-grouper-cpu face-grouper-gpu face-grouper-rocm 2>/dev/null || true

# =============================================================================
# Test
# =============================================================================

test: test-go test-docker
	@echo "$(GREEN)✓ All tests passed$(NC)"

test-go:
	@echo "$(BLUE)Running Go unit tests...$(NC)"
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "$(GREEN)Coverage report: coverage.html$(NC)"

test-docker: test-docker-cpu test-docker-gpu test-docker-rocm
	@echo "$(GREEN)✓ All Docker tests passed$(NC)"

test-docker-cpu:
	@echo "$(BLUE)Testing CPU image...$(NC)"
	docker run --rm \
		-v $(PWD)/test-dataset:/app/dataset:ro \
		-v $(PWD)/test-output:/app/output \
		-v $(PWD)/test-models:/app/models:ro \
		$(IMAGE_NAME):cpu --help || true
	mkdir -p test-dataset test-output test-models
	rm -rf test-dataset test-output test-models

test-docker-gpu:
	@echo "$(BLUE)Testing GPU image...$(NC)"
	@if docker run --rm --gpus all nvidia/cuda:12.2.0-cudnn8-runtime-ubuntu22.04 nvidia-smi > /dev/null 2>&1; then \
		mkdir -p test-dataset test-output test-models && \
		docker run --rm --gpus all \
			-v $(PWD)/test-dataset:/app/dataset:ro \
			-v $(PWD)/test-output:/app/output \
			-v $(PWD)/test-models:/app/models:ro \
			$(IMAGE_NAME):gpu --help || true && \
		rm -rf test-dataset test-output test-models; \
	else \
		echo "$(YELLOW)NVIDIA GPU not available, skipping GPU test$(NC)"; \
	fi

test-docker-rocm:
	@echo "$(BLUE)Testing ROCm image...$(NC)"
	@if [ -e /dev/kfd ] && [ -e /dev/dri ]; then \
		mkdir -p test-dataset test-output test-models && \
		docker run --rm --device /dev/kfd --device /dev/dri --group-add video \
			-v $(PWD)/test-dataset:/app/dataset:ro \
			-v $(PWD)/test-output:/app/output \
			-v $(PWD)/test-models:/app/models:ro \
			$(IMAGE_NAME):rocm --help || true && \
		rm -rf test-dataset test-output test-models; \
	else \
		echo "$(YELLOW)ROCm devices not available, skipping ROCm test$(NC)"; \
	fi

# =============================================================================
# Push
# =============================================================================

push: push-cpu push-gpu push-rocm
	@echo "$(GREEN)✓ All images pushed$(NC)"

push-cpu:
	@echo "$(BLUE)Pushing CPU image...$(NC)"
	docker push $(REGISTRY)/$(IMAGE_NAME):cpu
	docker push $(REGISTRY)/$(IMAGE_NAME):cpu-$(VERSION)

push-gpu:
	@echo "$(BLUE)Pushing GPU image...$(NC)"
	docker push $(REGISTRY)/$(IMAGE_NAME):gpu
	docker push $(REGISTRY)/$(IMAGE_NAME):gpu-$(VERSION)

push-rocm:
	@echo "$(BLUE)Pushing ROCm image...$(NC)"
	docker push $(REGISTRY)/$(IMAGE_NAME):rocm
	docker push $(REGISTRY)/$(IMAGE_NAME):rocm-$(VERSION)

# =============================================================================
# Security Scanning
# =============================================================================

scan: scan-cpu scan-gpu scan-rocm
	@echo "$(GREEN)✓ All scans completed$(NC)"

scan-cpu:
	@echo "$(BLUE)Scanning CPU image for vulnerabilities...$(NC)"
	trivy image --severity HIGH,CRITICAL $(IMAGE_NAME):cpu

scan-gpu:
	@echo "$(BLUE)Scanning GPU image for vulnerabilities...$(NC)"
	trivy image --severity HIGH,CRITICAL $(IMAGE_NAME):gpu

scan-rocm:
	@echo "$(BLUE)Scanning ROCm image for vulnerabilities...$(NC)"
	trivy image --severity HIGH,CRITICAL $(IMAGE_NAME):rocm

# =============================================================================
# Benchmark
# =============================================================================

benchmark:
	@echo "$(BLUE)Running CPU vs GPU benchmark...$(NC)"
	@./scripts/benchmark.sh

# =============================================================================
# Cleanup
# =============================================================================

clean:
	@echo "$(YELLOW)Removing containers...$(NC)"
	docker-compose down || true
	docker stop face-grouper-cpu face-grouper-gpu face-grouper-rocm 2>/dev/null || true
	docker rm face-grouper-cpu face-grouper-gpu face-grouper-rocm 2>/dev/null || true
	@echo "$(YELLOW)Removing images...$(NC)"
	docker rmi $(IMAGE_NAME):cpu $(IMAGE_NAME):gpu $(IMAGE_NAME):rocm 2>/dev/null || true
	docker rmi $(REGISTRY)/$(IMAGE_NAME):cpu $(REGISTRY)/$(IMAGE_NAME):gpu $(REGISTRY)/$(IMAGE_NAME):rocm 2>/dev/null || true
	@echo "$(GREEN)✓ Cleanup completed$(NC)"

clean-volumes:
	@echo "$(YELLOW)Removing output volumes...$(NC)"
	sudo rm -rf ./output/*
	@echo "$(GREEN)✓ Volumes cleaned$(NC)"

# =============================================================================
# Development
# =============================================================================

dev:
	@echo "$(BLUE)Starting development environment...$(NC)"
	docker-compose up -d face-grouper-cpu
	docker-compose logs -f

logs:
	docker-compose logs -f

shell:
	docker-compose exec face-grouper-cpu /bin/bash
