# Face Grouper - Docker Deployment Guide

Complete guide for deploying Face Grouper using Docker.

**Main Documentation:** [README.md](README.md)

## Table of Contents

- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Building Images](#building-images)
- [Running Containers](#running-containers)
- [Configuration](#configuration)
- [GPU Support](#gpu-support)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

### Required Software

| Software | Version | Purpose |
|----------|---------|---------|
| Docker | 20.10+ | Container runtime |
| Docker Compose | 2.0+ | Multi-container orchestration |
| NVIDIA Driver | 450.80.02+ | NVIDIA GPU support (optional) |
| NVIDIA Container Toolkit | 1.9+ | NVIDIA GPU in Docker (optional) |
| AMD ROCm | 5.4+ | AMD GPU support (optional) |

### NVIDIA GPU Setup (Optional)

For GPU acceleration, install the **NVIDIA Container Toolkit**:

```bash
# Ubuntu/Debian
distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
curl -s -L https://nvidia.github.io/nvidia-docker/gpgkey | sudo apt-key add -
curl -s -L https://nvidia.github.io/nvidia-docker/$distribution/nvidia-docker.list | \
  sudo tee /etc/apt/sources.list.d/nvidia-docker.list

sudo apt-get update
sudo apt-get install -y nvidia-container-toolkit
sudo systemctl restart docker

# Verify installation
docker run --rm --gpus all nvidia/cuda:12.2.0-cudnn8-runtime-ubuntu22.04 nvidia-smi
```

### AMD ROCm Setup (Optional)

For AMD GPU acceleration, install **ROCm** and configure Docker:

```bash
# Ubuntu/Debian - Install ROCm
sudo apt update
sudo apt install -y rocm-dkms rocm-opencl-runtime rocm-ml-sdk

# Add user to video and render groups
sudo usermod -a -G video $USER
sudo usermod -a -G render $USER

# Logout and login for group changes to take effect

# Verify ROCm installation
rocm-smi
rocminfo

# Docker should automatically have access to /dev/kfd and /dev/dri
# Test with:
docker run --device /dev/kfd --device /dev/dri --group-add video \
  rocm/pytorch:rocm6.0-ubuntu22.04 rocminfo
```

**Supported AMD GPUs:**
- RX 5000 series (Navi 10)
- RX 6000 series (Navi 20)
- RX 7000 series (Navi 30)
- MI50, MI100, MI200, MI300 (Datacenter)

For full compatibility list, see: https://rocm.docs.amd.com/projects/install-on-linux/en/latest/reference/system-requirements.html

---

## Quick Start

### 1. Clone and Prepare

```bash
# Clone repository
git clone https://github.com/kont1n/face-grouper.git
cd face-grouper

# Create necessary directories
mkdir -p dataset output models

# Download models (see QUICKSTART.md for instructions)
# Place det_10g.onnx and w600k_r50.onnx in ./models/

# Add photos to ./dataset/
```

### 2. Run with Docker Compose

```bash
# Navigate to docker directory
cd deploy/docker

# CPU version
docker-compose up -d face-grouper-cpu

# NVIDIA GPU version (requires NVIDIA GPU)
docker-compose up -d face-grouper-gpu

# AMD ROCm version (requires AMD GPU)
docker-compose up -d face-grouper-rocm

# View logs
docker-compose logs -f

# Stop
docker-compose down
```

### 3. Access Web UI

Open browser:
- CPU: http://localhost:8080
- NVIDIA GPU: http://localhost:8081
- AMD ROCm: http://localhost:8082

---

## Building Images

### Build CPU Image

```bash
docker build -t face-grouper:cpu -f deploy/docker/Dockerfile .
```

### Build GPU Image (NVIDIA)

```bash
docker build -t face-grouper:gpu -f deploy/docker/Dockerfile.nvidia .
```

### Build GPU Image (AMD ROCm)

```bash
docker build -t face-grouper:rocm -f deploy/docker/Dockerfile.rocm .
```

### Build with Custom ONNX Runtime Version

```bash
docker build -t face-grouper:cpu \
  --build-arg ONNXRUNTIME_VERSION=1.24.0 \
  -f Dockerfile .
```

### Multi-Platform Build

```bash
docker buildx build --platform linux/amd64,linux/arm64 \
  -t face-grouper:cpu \
  -f Dockerfile .
```

---

## Running Containers

### Using Docker Run

#### CPU Version

```bash
docker run -d \
  --name face-grouper-cpu \
  -v $(pwd)/dataset:/app/dataset:ro \
  -v $(pwd)/output:/app/output \
  -v $(pwd)/models:/app/models:ro \
  -p 8080:8080 \
  -e GPU_ENABLED=0 \
  -e EXTRACT_WORKERS=4 \
  face-grouper:cpu
```

#### GPU Version (NVIDIA)

```bash
docker run -d \
  --name face-grouper-gpu \
  --gpus all \
  -v $(pwd)/dataset:/app/dataset:ro \
  -v $(pwd)/output:/app/output \
  -v $(pwd)/models:/app/models:ro \
  -p 8081:8080 \
  -e GPU_ENABLED=1 \
  -e GPU_DEVICE_ID=0 \
  face-grouper:gpu
```

#### GPU Version (AMD ROCm)

```bash
docker run -d \
  --name face-grouper-rocm \
  --device /dev/kfd \
  --device /dev/dri \
  --group-add video \
  --group-add render \
  -v $(pwd)/dataset:/app/dataset:ro \
  -v $(pwd)/output:/app/output \
  -v $(pwd)/models:/app/models:ro \
  -p 8082:8080 \
  -e GPU_ENABLED=1 \
  -e GPU_DEVICE_ID=0 \
  -e PROVIDER_PRIORITY=rocm \
  face-grouper:rocm
```

### Using Docker Compose

```bash
# Start services
docker-compose up -d

# Start specific service
docker-compose up -d face-grouper-cpu

# View logs
docker-compose logs -f face-grouper-cpu

# Stop services
docker-compose down

# Stop and remove volumes
docker-compose down -v

# Rebuild and restart
docker-compose up -d --build
```

---

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `INPUT_DIR` | `/app/dataset` | Input directory with photos |
| `OUTPUT_DIR` | `/app/output` | Output directory for results |
| `MODELS_DIR` | `/app/models` | Directory with ONNX models |
| `GPU_ENABLED` | `0` | Enable GPU (1) or CPU (0) |
| `GPU_DEVICE_ID` | `0` | GPU device ID for multi-GPU |
| `FORCE_CPU` | `0` | Force CPU usage |
| `PROVIDER_PRIORITY` | `auto` | Provider: auto, cpu, cuda, rocm |
| `EXTRACT_WORKERS` | `4` | Number of extraction workers |
| `GPU_DET_SESSIONS` | `2` | GPU detector sessions |
| `GPU_REC_SESSIONS` | `2` | GPU recognizer sessions |
| `EMBED_BATCH_SIZE` | `64` | Embedding batch size |
| `LOG_LEVEL` | `info` | Log level: debug, info, warn, error |
| `LOG_JSON` | `false` | JSON format logs |
| `WEB_SERVE` | `true` | Enable web UI |
| `WEB_PORT` | `8080` | Web UI port |

### Example .env File

```bash
# Create .env file
cat > .env << EOF
INPUT_DIR=/app/dataset
OUTPUT_DIR=/app/output
MODELS_DIR=/app/models
GPU_ENABLED=1
GPU_DEVICE_ID=0
PROVIDER_PRIORITY=cuda
EXTRACT_WORKERS=4
EMBED_BATCH_SIZE=64
LOG_LEVEL=info
WEB_SERVE=true
WEB_PORT=8080
EOF

# Run with .env
docker run --env-file .env face-grouper:gpu
```

---

## GPU Support

### NVIDIA GPU

**Requirements:**
- NVIDIA GPU with Compute Capability 5.0+
- NVIDIA Driver 450.80.02+
- NVIDIA Container Toolkit

**Run with specific GPU:**

```bash
# Use GPU 0
docker run -d --gpus '"device=0"' face-grouper:gpu

# Use GPUs 0 and 1
docker run -d --gpus '"device=0,1"' face-grouper:gpu

# Use all GPUs
docker run -d --gpus all face-grouper:gpu
```

**Monitor GPU usage:**

```bash
# On host
watch -n 1 nvidia-smi

# In container
docker exec face-grouper-gpu nvidia-smi
```

### AMD ROCm

**Requirements:**
- AMD GPU with GCN 4.0+ (RDNA2/CDNA2+ recommended)
- ROCm 5.4+ (6.0 recommended)
- Docker with device access (/dev/kfd, /dev/dri)

**Supported GPUs:**
- RX 5000 series (Navi 10)
- RX 6000 series (Navi 20)
- RX 7000 series (Navi 30)
- MI50, MI100, MI200, MI300 (Datacenter)

**Run with specific GPU:**

```bash
# Use GPU 0
docker run -d --device /dev/kfd --device /dev/dri --group-add video \
  -e HIP_VISIBLE_DEVICES=0 \
  face-grouper:rocm

# Use GPU 1
docker run -d --device /dev/kfd --device /dev/dri --group-add video \
  -e HIP_VISIBLE_DEVICES=1 \
  face-grouper:rocm
```

**Monitor GPU usage:**

```bash
# On host
watch -n 1 rocm-smi
rocminfo

# In container
docker exec face-grouper-rocm rocminfo
```

**Troubleshooting ROCm:**

```bash
# Check ROCm installation
rocm-smi
rocminfo

# Check user groups
groups $USER  # Should include 'video' and 'render'

# Check device access
ls -la /dev/kfd /dev/dri

# Test Docker ROCm access
docker run --device /dev/kfd --device /dev/dri --group-add video \
  rocm/pytorch:rocm6.0-ubuntu22.04 rocminfo
```
---

## Troubleshooting

### Common Issues

#### 1. "Cannot find ONNX Runtime library"

**Solution:** Ensure ONNX Runtime DLL is in the correct location:

```bash
# Check library path
docker exec face-grouper-cpu ls -la /opt/onnxruntime/lib/

# Rebuild image
docker-compose build --no-cache face-grouper-cpu
```

#### 2. "CUDA error: cudaErrorNoKernelImageForDevice"

**Cause:** GPU compute capability not supported by ONNX Runtime version.

**Solution:** Use compatible ONNX Runtime version:

```bash
docker build -t face-grouper:gpu \
  --build-arg ONNXRUNTIME_VERSION=1.23.0 \
  -f Dockerfile.nvidia .
```

#### 3. "NVIDIA-SMI has failed"

**Cause:** NVIDIA Container Toolkit not installed or configured.

**Solution:**

```bash
# Install NVIDIA Container Toolkit
distribution=$(. /etc/os-release;echo $ID$VERSION_ID)
curl -s -L https://nvidia.github.io/nvidia-docker/gpgkey | sudo apt-key add -
curl -s -L https://nvidia.github.io/nvidia-docker/$distribution/nvidia-docker.list | \
  sudo tee /etc/apt/sources.list.d/nvidia-docker.list
sudo apt-get update
sudo apt-get install -y nvidia-container-toolkit
sudo systemctl restart docker
```

#### 4. "Out of memory"

**Solution:** Reduce batch size and workers:

```bash
docker run -d \
  -e EMBED_BATCH_SIZE=32 \
  -e EXTRACT_WORKERS=2 \
  face-grouper:gpu
```

#### 5. "Permission denied" on mounted volumes

**Solution:** Fix permissions:

```bash
sudo chown -R 1000:1000 ./dataset ./output ./models
```

### Debug Mode

```bash
# Enable debug logging
docker run -d \
  -e LOG_LEVEL=debug \
  -e LOG_JSON=false \
  face-grouper:cpu

# View detailed logs
docker logs -f face-grouper-cpu 2>&1 | grep -i error
```

### Performance Tuning

| Parameter | CPU | GPU (RTX 3060+) | GPU (RTX 4090+) |
|-----------|-----|-----------------|-----------------|
| `EXTRACT_WORKERS` | 4 | 8 | 16 |
| `EMBED_BATCH_SIZE` | 32 | 64 | 128 |
| `GPU_DET_SESSIONS` | N/A | 2 | 4 |
| `GPU_REC_SESSIONS` | N/A | 2 | 4 |

---

## Security Considerations

### Run as Non-Root User

```dockerfile
# Add to Dockerfile
RUN useradd -m -u 1000 appuser
USER appuser
```

### Read-Only Root Filesystem

```bash
docker run -d --read-only \
  --tmpfs /app/output \
  --tmpfs /tmp \
  face-grouper:cpu
```

### Network Isolation

```bash
docker run -d --network=none \
  --expose 8080 \
  face-grouper:cpu
```

---

## Advanced Usage

### Multi-GPU Processing

```yaml
# docker-compose.multi-gpu.yml
version: '3.8'
services:
  face-grouper-gpu-0:
    image: face-grouper:gpu
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              device_ids: ['0']
              capabilities: [gpu]
    environment:
      - GPU_DEVICE_ID=0
      - OUTPUT_DIR=/app/output_0
  
  face-grouper-gpu-1:
    image: face-grouper:gpu
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              device_ids: ['1']
              capabilities: [gpu]
    environment:
      - GPU_DEVICE_ID=1
      - OUTPUT_DIR=/app/output_1
```

### Kubernetes Deployment

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: face-grouper
spec:
  replicas: 3
  selector:
    matchLabels:
      app: face-grouper
  template:
    spec:
      containers:
      - name: face-grouper
        image: face-grouper:gpu
        resources:
          limits:
            nvidia.com/gpu: 1
            memory: 4Gi
          requests:
            nvidia.com/gpu: 1
            memory: 2Gi
        volumeMounts:
        - name: dataset
          mountPath: /app/dataset
          readOnly: true
        - name: output
          mountPath: /app/output
        - name: models
          mountPath: /app/models
          readOnly: true
      volumes:
      - name: dataset
        persistentVolumeClaim:
          claimName: dataset-pvc
      - name: output
        persistentVolumeClaim:
          claimName: output-pvc
      - name: models
        persistentVolumeClaim:
          claimName: models-pvc
```

---

## Support

- **Issues:** https://github.com/kont1n/face-grouper/issues
- **Discussions:** https://github.com/kont1n/face-grouper/discussions
