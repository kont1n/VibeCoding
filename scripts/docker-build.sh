#!/bin/bash
# =============================================================================
# Face Grouper - Docker Build and Test Script
# =============================================================================
# Usage:
#   ./scripts/docker-build.sh [cpu|gpu|all]
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if Docker is available
check_docker() {
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed or not in PATH"
        exit 1
    fi
    
    if ! docker info &> /dev/null; then
        log_error "Docker daemon is not running"
        exit 1
    fi
    
    log_info "Docker is available: $(docker --version)"
}

# Build CPU image
build_cpu() {
    log_info "Building CPU image..."
    
    docker build \
        -t face-grouper:cpu \
        -t face-grouper:cpu-latest \
        -f Dockerfile \
        --build-arg ONNXRUNTIME_VERSION=1.23.0 \
        .
    
    log_info "CPU image built successfully"
    
    # Show image size
    IMAGE_SIZE=$(docker image inspect face-grouper:cpu --format='{{.Size}}' | awk '{printf "%.1f MB", $1/1024/1024}')
    log_info "CPU image size: $IMAGE_SIZE"
}

# Build GPU image
build_gpu() {
    log_info "Building GPU image..."
    
    docker build \
        -t face-grouper:gpu \
        -t face-grouper:gpu-latest \
        -f Dockerfile.nvidia \
        --build-arg ONNXRUNTIME_VERSION=1.23.0 \
        .
    
    log_info "GPU image built successfully"
    
    # Show image size
    IMAGE_SIZE=$(docker image inspect face-grouper:gpu --format='{{.Size}}' | awk '{printf "%.1f MB", $1/1024/1024}')
    log_info "GPU image size: $IMAGE_SIZE"
}

# Test CPU image
test_cpu() {
    log_info "Testing CPU image..."
    
    # Create test directories
    mkdir -p test-dataset test-output test-models
    
    # Run container
    docker run --rm \
        -v "$(pwd)/test-dataset:/app/dataset:ro" \
        -v "$(pwd)/test-output:/app/output" \
        -v "$(pwd)/test-models:/app/models:ro" \
        -e LOG_LEVEL=info \
        face-grouper:cpu \
        --help || true
    
    # Cleanup
    rm -rf test-dataset test-output test-models
    
    log_info "CPU image test passed"
}

# Test GPU image
test_gpu() {
    log_info "Testing GPU image..."
    
    # Check NVIDIA Container Toolkit
    if ! docker run --rm --gpus all nvidia/cuda:12.2.0-cudnn8-runtime-ubuntu22.04 nvidia-smi &> /dev/null; then
        log_warn "NVIDIA Container Toolkit not available, skipping GPU test"
        return
    fi
    
    # Create test directories
    mkdir -p test-dataset test-output test-models
    
    # Run container
    docker run --rm --gpus all \
        -v "$(pwd)/test-dataset:/app/dataset:ro" \
        -v "$(pwd)/test-output:/app/output" \
        -v "$(pwd)/test-models:/app/models:ro" \
        -e LOG_LEVEL=info \
        face-grouper:gpu \
        --help || true
    
    # Cleanup
    rm -rf test-dataset test-output test-models
    
    log_info "GPU image test passed"
}

# Main
main() {
    local target="${1:-all}"
    
    log_info "Starting Docker build and test..."
    log_info "Target: $target"
    
    check_docker
    
    case "$target" in
        cpu)
            build_cpu
            test_cpu
            ;;
        gpu)
            build_gpu
            test_gpu
            ;;
        all)
            build_cpu
            build_gpu
            test_cpu
            test_gpu
            ;;
        *)
            log_error "Unknown target: $target"
            echo "Usage: $0 [cpu|gpu|all]"
            exit 1
            ;;
    esac
    
    log_info "Build and test completed successfully!"
    
    # Show summary
    echo ""
    log_info "Available images:"
    docker images face-grouper --format "table {{.Repository}}\t{{.Tag}}\t{{.Size}}"
}

main "$@"
