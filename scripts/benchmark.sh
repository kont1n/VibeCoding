#!/bin/bash
# =============================================================================
# Face Grouper - Performance Benchmark Script
# =============================================================================
# Compares CPU vs GPU (NVIDIA/AMD) performance
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_DIR"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_step() { echo -e "${BLUE}[STEP]${NC} $1"; }

# Configuration
DATASET_SIZE=${DATASET_SIZE:-"100"}  # Number of images to process
OUTPUT_DIR="./benchmark-output"
RESULTS_FILE="./benchmark-results.md"

# Check prerequisites
check_prerequisites() {
    log_step "Checking prerequisites..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker is not installed"
        exit 1
    fi
    
    if ! command -v docker-compose &> /dev/null; then
        log_error "docker-compose is not installed"
        exit 1
    fi
    
    log_info "Prerequisites OK"
}

# Prepare test dataset
prepare_dataset() {
    log_step "Preparing test dataset ($DATASET_SIZE images)..."
    
    mkdir -p "$OUTPUT_DIR"
    mkdir -p benchmark-dataset
    
    # Copy sample images from dataset if available
    if [ -d "./dataset" ] && [ "$(ls -A ./dataset 2>/dev/null)" ]; then
        ls ./dataset/*.jpg ./dataset/*.jpeg ./dataset/*.png 2>/dev/null | head -n "$DATASET_SIZE" | while read img; do
            cp "$img" benchmark-dataset/ 2>/dev/null || true
        done
        
        COPIED=$(ls benchmark-dataset | wc -l)
        if [ "$COPIED" -lt "$DATASET_SIZE" ]; then
            log_warn "Only $COPIED images available (requested $DATASET_SIZE)"
        fi
    else
        log_error "No images found in ./dataset"
        log_info "Please add images to ./dataset first"
        exit 1
    fi
    
    log_info "Dataset prepared: $(ls benchmark-dataset | wc -l) images"
}

# Run benchmark for specific provider
run_benchmark() {
    local provider=$1
    local image=$2
    local port=$3
    local extra_args=$4
    
    log_step "Running benchmark: $provider..."
    
    local start_time=$(date +%s.%N)
    
    # Run container
    docker run --rm \
        -v $(pwd)/benchmark-dataset:/app/dataset:ro \
        -v $(pwd)/$OUTPUT_DIR/$provider:/app/output \
        -v $(pwd)/models:/app/models:ro \
        -p $port:8080 \
        $extra_args \
        $image 2>&1 | tee $OUTPUT_DIR/${provider}-log.txt
    
    local end_time=$(date +%s.%N)
    local duration=$(echo "$end_time - $start_time" | bc)
    
    echo "$duration" > $OUTPUT_DIR/${provider}-time.txt
    
    log_info "$provider completed in ${duration}s"
}

# Benchmark CPU
benchmark_cpu() {
    run_benchmark "cpu" "face-grouper:cpu" "8080" "-e LOG_LEVEL=info"
}

# Benchmark NVIDIA GPU
benchmark_gpu() {
    if docker run --rm --gpus all nvidia/cuda:12.2.0-cudnn8-runtime-ubuntu22.04 nvidia-smi > /dev/null 2>&1; then
        run_benchmark "gpu-nvidia" "face-grouper:gpu" "8081" "--gpus all -e GPU_ENABLED=1 -e LOG_LEVEL=info"
    else
        log_warn "NVIDIA GPU not available, skipping GPU benchmark"
        echo "N/A" > $OUTPUT_DIR/gpu-nvidia-time.txt
    fi
}

# Benchmark AMD ROCm
benchmark_rocm() {
    if [ -e /dev/kfd ] && [ -e /dev/dri ]; then
        run_benchmark "gpu-rocm" "face-grouper:rocm" "8082" "--device /dev/kfd --device /dev/dri --group-add video --group-add render -e GPU_ENABLED=1 -e PROVIDER_PRIORITY=rocm"
    else
        log_warn "AMD ROCm not available, skipping ROCm benchmark"
        echo "N/A" > $OUTPUT_DIR/gpu-rocm-time.txt
    fi
}

# Generate results report
generate_report() {
    log_step "Generating benchmark report..."
    
    local cpu_time=$(cat $OUTPUT_DIR/cpu-time.txt 2>/dev/null || echo "N/A")
    local gpu_nvidia_time=$(cat $OUTPUT_DIR/gpu-nvidia-time.txt 2>/dev/null || echo "N/A")
    local gpu_rocm_time=$(cat $OUTPUT_DIR/gpu-rocm-time.txt 2>/dev/null || echo "N/A")
    
    # Calculate speedup
    local gpu_nvidia_speedup="N/A"
    local gpu_rocm_speedup="N/A"
    
    if [ "$cpu_time" != "N/A" ] && [ "$gpu_nvidia_time" != "N/A" ]; then
        gpu_nvidia_speedup=$(echo "scale=2; $cpu_time / $gpu_nvidia_time" | bc)
    fi
    
    if [ "$cpu_time" != "N/A" ] && [ "$gpu_rocm_time" != "N/A" ]; then
        gpu_rocm_speedup=$(echo "scale=2; $cpu_time / $gpu_rocm_time" | bc)
    fi
    
    cat > "$RESULTS_FILE" << EOF
# Face Grouper - Performance Benchmark Results

## Test Configuration

- **Dataset Size:** $DATASET_SIZE images
- **Date:** $(date -u +"%Y-%m-%d %H:%M:%S UTC")
- **Git Commit:** $(git rev-parse --short HEAD 2>/dev/null || echo "N/A")
- **ONNX Runtime Version:** 1.23.0

## Hardware

### CPU
- **Model:** $(grep "model name" /proc/cpuinfo | head -1 | cut -d: -f2 | xargs || echo "Unknown")
- **Cores:** $(nproc)

### GPU (NVIDIA)
EOF

    if [ "$gpu_nvidia_time" != "N/A" ]; then
        echo "- **Model:** $(nvidia-smi --query-gpu=gpu_name --format=csv,noheader | head -1 || echo "Unknown")" >> "$RESULTS_FILE"
        echo "- **Driver:** $(nvidia-smi --query-gpu=driver_version --format=csv,noheader | head -1 || echo "Unknown")" >> "$RESULTS_FILE"
        echo "- **CUDA:** $(nvidia-smi --query-gpu=cuda_version --format=csv,noheader | head -1 || echo "Unknown")" >> "$RESULTS_FILE"
    else
        echo "- **Status:** Not available" >> "$RESULTS_FILE"
    fi
    
    cat >> "$RESULTS_FILE" << EOF

### GPU (AMD ROCm)
EOF

    if [ "$gpu_rocm_time" != "N/A" ]; then
        echo "- **Model:** $(rocm-smi --showproductname 2>/dev/null | grep -v "GPU" | head -1 || echo "Unknown")" >> "$RESULTS_FILE"
        echo "- **ROCm Version:** $(rocminfo | grep "Version:" | head -1 || echo "Unknown")" >> "$RESULTS_FILE"
    else
        echo "- **Status:** Not available" >> "$RESULTS_FILE"
    fi
    
    cat >> "$RESULTS_FILE" << EOF

## Results

| Provider | Time (seconds) | Speedup vs CPU |
|----------|---------------|----------------|
| CPU | $cpu_time | 1.0x |
| NVIDIA GPU | $gpu_nvidia_time | ${gpu_nvidia_speedup}x |
| AMD ROCm | $gpu_rocm_time | ${gpu_rocm_speedup}x |

## Analysis

EOF

    if [ "$gpu_nvidia_speedup" != "N/A" ]; then
        if (( $(echo "$gpu_nvidia_speedup > 2" | bc -l) )); then
            echo "✅ **NVIDIA GPU shows significant speedup** (${gpu_nvidia_speedup}x faster than CPU)" >> "$RESULTS_FILE"
        elif (( $(echo "$gpu_nvidia_speedup > 1" | bc -l) )); then
            echo "⚠️ **NVIDIA GPU shows moderate speedup** (${gpu_nvidia_speedup}x faster than CPU)" >> "$RESULTS_FILE"
        else
            echo "❌ **NVIDIA GPU slower than CPU** (${gpu_nvidia_speedup}x)" >> "$RESULTS_FILE"
        fi
    fi
    
    if [ "$gpu_rocm_speedup" != "N/A" ]; then
        if (( $(echo "$gpu_rocm_speedup > 2" | bc -l) )); then
            echo "✅ **AMD ROCm shows significant speedup** (${gpu_rocm_speedup}x faster than CPU)" >> "$RESULTS_FILE"
        elif (( $(echo "$gpu_rocm_speedup > 1" | bc -l) )); then
            echo "⚠️ **AMD ROCm shows moderate speedup** (${gpu_rocm_speedup}x faster than CPU)" >> "$RESULTS_FILE"
        else
            echo "❌ **AMD ROCm slower than CPU** (${gpu_rocm_speedup}x)" >> "$RESULTS_FILE"
        fi
    fi
    
    cat >> "$RESULTS_FILE" << EOF

## Recommendations

Based on the benchmark results:

1. **For large datasets (1000+ images):** Use GPU acceleration
2. **For small datasets (<100 images):** CPU may be sufficient
3. **For production:** Consider auto-scaling based on queue size

## How to Run

\`\`\`bash
# Run benchmark
./scripts/benchmark.sh

# View results
cat benchmark-results.md

# Clean up
rm -rf benchmark-dataset benchmark-output
\`\`\`

---
*Generated by Face Grouper Benchmark Script*
EOF

    log_info "Report generated: $RESULTS_FILE"
}

# Cleanup
cleanup() {
    log_step "Cleaning up..."
    # Keep results but remove temporary files
    # rm -rf benchmark-dataset
    log_info "Benchmark data preserved in $OUTPUT_DIR"
}

# Main
main() {
    log_info "Face Grouper - Performance Benchmark"
    log_info "===================================="
    
    check_prerequisites
    
    # Build images if not present
    if ! docker image inspect face-grouper:cpu > /dev/null 2>&1; then
        log_step "Building CPU image..."
        make build-cpu
    fi
    
    if ! docker image inspect face-grouper:gpu > /dev/null 2>&1; then
        log_step "Building GPU image..."
        make build-gpu
    fi
    
    if ! docker image inspect face-grouper:rocm > /dev/null 2>&1; then
        log_step "Building ROCm image..."
        make build-rocm
    fi
    
    prepare_dataset
    mkdir -p "$OUTPUT_DIR"
    
    benchmark_cpu
    benchmark_gpu
    benchmark_rocm
    
    generate_report
    cleanup
    
    log_info ""
    log_info "Benchmark completed!"
    log_info "Results: $RESULTS_FILE"
    
    # Display summary
    echo ""
    echo "=== Summary ==="
    cat "$RESULTS_FILE" | grep -A 10 "^## Results"
}

main "$@"
