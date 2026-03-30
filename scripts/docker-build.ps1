# =============================================================================
# Face Grouper - Docker Build and Test Script (PowerShell)
# =============================================================================
# Usage:
#   .\scripts\docker-build.ps1 [cpu|gpu|all]
# =============================================================================

param(
    [ValidateSet("cpu", "gpu", "all")]
    [string]$Target = "all"
)

$ErrorActionPreference = "Stop"

# Colors
function Write-Info { Write-Host "[INFO] $args" -ForegroundColor Green }
function Write-Warn { Write-Host "[WARN] $args" -ForegroundColor Yellow }
function Write-Error { Write-Host "[ERROR] $args" -ForegroundColor Red }

# Check Docker
function Test-Docker {
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
        Write-Error "Docker is not installed or not in PATH"
        exit 1
    }
    
    try {
        docker info | Out-Null
        Write-Info "Docker is available: $(docker --version)"
    } catch {
        Write-Error "Docker daemon is not running"
        exit 1
    }
}

# Build CPU image
function Build-CPU {
    Write-Info "Building CPU image..."
    
    docker build `
        -t face-grouper:cpu `
        -t face-grouper:cpu-latest `
        -f Dockerfile `
        --build-arg ONNXRUNTIME_VERSION=1.23.0 `
        .
    
    if ($LASTEXITCODE -ne 0) {
        Write-Error "CPU image build failed"
        exit 1
    }
    
    Write-Info "CPU image built successfully"
    
    # Show image size
    $image = docker image inspect face-grouper:cpu --format='{{.Size}}'
    $sizeMB = [math]::Round($image / 1MB, 1)
    Write-Info "CPU image size: $sizeMB MB"
}

# Build GPU image
function Build-GPU {
    Write-Info "Building GPU image..."
    
    docker build `
        -t face-grouper:gpu `
        -t face-grouper:gpu-latest `
        -f Dockerfile.nvidia `
        --build-arg ONNXRUNTIME_VERSION=1.23.0 `
        .
    
    if ($LASTEXITCODE -ne 0) {
        Write-Error "GPU image build failed"
        exit 1
    }
    
    Write-Info "GPU image built successfully"
    
    # Show image size
    $image = docker image inspect face-grouper:gpu --format='{{.Size}}'
    $sizeMB = [math]::Round($image / 1MB, 1)
    Write-Info "GPU image size: $sizeMB MB"
}

# Test CPU image
function Test-CPU {
    Write-Info "Testing CPU image..."
    
    # Create test directories
    $testDirs = @("test-dataset", "test-output", "test-models")
    foreach ($dir in $testDirs) {
        if (-not (Test-Path $dir)) {
            New-Item -ItemType Directory -Path $dir | Out-Null
        }
    }
    
    # Run container
    docker run --rm `
        -v "${PWD}/test-dataset:/app/dataset:ro" `
        -v "${PWD}/test-output:/app/output" `
        -v "${PWD}/test-models:/app/models:ro" `
        -e LOG_LEVEL=info `
        face-grouper:cpu `
        --help 2>&1 | Out-Null
    
    # Cleanup
    foreach ($dir in $testDirs) {
        Remove-Item -Recurse -Force $dir -ErrorAction SilentlyContinue
    }
    
    Write-Info "CPU image test passed"
}

# Test GPU image
function Test-GPU {
    Write-Info "Testing GPU image..."
    
    # Check NVIDIA Container Toolkit
    $nvidiaTest = docker run --rm --gpus all nvidia/cuda:12.2.2-cudnn8-runtime-ubuntu22.04 nvidia-smi 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Warn "NVIDIA Container Toolkit not available, skipping GPU test"
        return
    }
    
    # Create test directories
    $testDirs = @("test-dataset", "test-output", "test-models")
    foreach ($dir in $testDirs) {
        if (-not (Test-Path $dir)) {
            New-Item -ItemType Directory -Path $dir | Out-Null
        }
    }
    
    # Run container
    docker run --rm --gpus all `
        -v "${PWD}/test-dataset:/app/dataset:ro" `
        -v "${PWD}/test-output:/app/output" `
        -v "${PWD}/test-models:/app/models:ro" `
        -e LOG_LEVEL=info `
        face-grouper:gpu `
        --help 2>&1 | Out-Null
    
    # Cleanup
    foreach ($dir in $testDirs) {
        Remove-Item -Recurse -Force $dir -ErrorAction SilentlyContinue
    }
    
    Write-Info "GPU image test passed"
}

# Main
Write-Info "Starting Docker build and test..."
Write-Info "Target: $Target"

Test-Docker

switch ($Target) {
    "cpu" {
        Build-CPU
        Test-CPU
    }
    "gpu" {
        Build-GPU
        Test-GPU
    }
    "all" {
        Build-CPU
        Build-GPU
        Test-CPU
        Test-GPU
    }
}

Write-Info "Build and test completed successfully!"

# Show summary
Write-Host ""
Write-Info "Available images:"
docker images face-grouper --format "table {{.Repository}}`t{{.Tag}}`t{{.Size}}"
