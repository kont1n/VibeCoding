# Face Grouper - Documentation

Welcome to the Face Grouper documentation hub.

## 📚 Documentation Index

### Getting Started

| Document | Description |
|----------|-------------|
| [Quick Start](QUICKSTART.md) | Get up and running in 5 minutes |
| [Download Models](DOWNLOAD_MODELS.md) | How to download InsightFace models |
| [README](../README.md) | Main project documentation |

### Deployment

| Document | Description |
|----------|-------------|
| [Docker Guide](DOCKER.md) | Complete Docker deployment guide |
| [GPU Setup](#gpu-setup) | NVIDIA GPU configuration |
| [AMD ROCm](#amd-rocm) | AMD GPU configuration |

### Development

| Document | Description |
|----------|-------------|
| [Architecture](ARCHITECTURE.md) | System architecture overview |
| [API Reference](API.md) | API documentation |
| [Testing Guide](TESTING.md) | Testing best practices |

---

## 🚀 Quick Links

**New to Face Grouper?**
1. Start with [Quick Start](QUICKSTART.md)
2. Download models using [Download Guide](DOWNLOAD_MODELS.md)
3. Run your first grouping!

**Want to use Docker?**
- Check out the [Docker Guide](DOCKER.md)
- Supports CPU, NVIDIA GPU, and AMD ROCm

**Developer?**
- Read [Architecture](ARCHITECTURE.md) docs
- Check [Testing Guide](TESTING.md)
- Review [API Reference](API.md)

---

## 📖 Documentation Structure

```
docs/
├── README.md                 # This file - Documentation index
├── QUICKSTART.md             # Quick start guide
├── DOWNLOAD_MODELS.md        # Model download instructions
├── DOCKER.md                 # Docker deployment guide
├── ARCHITECTURE.md           # System architecture (TODO)
├── API.md                    # API reference (TODO)
└── TESTING.md                # Testing guide (TODO)
```

---

## 🔧 Quick Reference

### Basic Commands

```bash
# Build
task build

# Test
task test

# Lint
task lint

# Docker build
task build:all

# Run Docker
task docker:run
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `INPUT_DIR` | `./dataset` | Input directory |
| `OUTPUT_DIR` | `./output` | Output directory |
| `MODELS_DIR` | `./models` | Models directory |
| `GPU_ENABLED` | `0` | Enable GPU |
| `EXTRACT_WORKERS` | `4` | Worker count |

---

## 📞 Support

- **Issues:** [GitHub Issues](https://github.com/kont1n/face-grouper/issues)
- **Discussions:** [GitHub Discussions](https://github.com/kont1n/face-grouper/discussions)
- **Main README:** [../README.md](../README.md)

---

*Last updated: March 2026*
