#!/bin/bash
# =============================================================================
# Face Grouper - ROCm Installation Script
# =============================================================================
# Installs AMD ROCm drivers and dependencies for Face Grouper
# Supports Ubuntu 20.04, 22.04 and Debian 11, 12
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Check if running as root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        log_error "Please run as root (sudo ./install-rocm.sh)"
        exit 1
    fi
}

# Detect OS
detect_os() {
    if [ -f /etc/os-release ]; then
        . /etc/os-release
        OS=$ID
        VERSION=$VERSION_ID
        log_info "Detected: $OS $VERSION"
    else
        log_error "Cannot detect OS"
        exit 1
    fi
}

# Check GPU compatibility
check_gpu() {
    log_info "Checking for AMD GPU..."
    
    # Check lspci
    if command -v lspci &> /dev/null; then
        AMD_GPU=$(lspci | grep -i 'vga.*amd\|vga.*advanced micro devices' || true)
        if [ -n "$AMD_GPU" ]; then
            log_info "AMD GPU detected:"
            echo "$AMD_GPU"
        else
            log_warn "No AMD GPU detected in lspci output"
            log_warn "ROCm may still work with external GPUs or datacenter GPUs"
        fi
    fi
}

# Install ROCm on Ubuntu/Debian
install_rocm_ubuntu() {
    log_info "Installing ROCm for Ubuntu/Debian..."
    
    # Add ROCm repository
    log_info "Adding ROCm repository..."
    wget -q -O - https://repo.radeon.com/rocm/rocm.gpg.key | apt-key add -
    
    # Determine repository URL based on Ubuntu version
    case "$VERSION" in
        22.04|22.04.*)
            echo "deb [arch=amd64] https://repo.radeon.com/rocm/apt/6.0 jammy main" > /etc/apt/sources.list.d/rocm.list
            log_info "Using ROCm 6.0 for Ubuntu 22.04"
            ;;
        20.04|20.04.*)
            echo "deb [arch=amd64] https://repo.radeon.com/rocm/apt/6.0 focal main" > /etc/apt/sources.list.d/rocm.list
            log_info "Using ROCm 6.0 for Ubuntu 20.04"
            ;;
        12|12.*)
            echo "deb [arch=amd64] https://repo.radeon.com/rocm/apt/6.0 bookworm main" > /etc/apt/sources.list.d/rocm.list
            log_info "Using ROCm 6.0 for Debian 12"
            ;;
        11|11.*)
            echo "deb [arch=amd64] https://repo.radeon.com/rocm/apt/6.0 bullseye main" > /etc/apt/sources.list.d/rocm.list
            log_info "Using ROCm 6.0 for Debian 11"
            ;;
        *)
            log_error "Unsupported version: $VERSION"
            exit 1
            ;;
    esac
    
    # Prefer ROCm over Mesa
    cat > /etc/apt/preferences.d/rocm-pin-600 << EOF
Package: *
Pin: release o=repo.radeon.com
Pin-Priority: 600
EOF
    
    # Update and install
    log_info "Updating package lists..."
    apt-get update
    
    log_info "Installing ROCm packages..."
    apt-get install -y rocm-dkms rocm-opencl-runtime rocm-ml-sdk rocm-smi-lib
    
    log_info "ROCm installation completed!"
}

# Configure user groups
configure_groups() {
    log_info "Configuring user groups..."
    
    # Get current user
    CURRENT_USER=${SUDO_USER:-$USER}
    
    # Add to video and render groups
    usermod -a -G video "$CURRENT_USER"
    usermod -a -G render "$CURRENT_USER"
    
    log_info "User $CURRENT_USER added to 'video' and 'render' groups"
    log_warn "Please logout and login again for group changes to take effect"
}

# Configure Docker
configure_docker() {
    log_info "Configuring Docker for ROCm..."
    
    # Check if Docker is installed
    if ! command -v docker &> /dev/null; then
        log_warn "Docker not installed. Install Docker first."
        return
    fi
    
    # Docker should automatically have access to /dev/kfd and /dev/dri
    # No additional configuration needed
    
    log_info "Docker ROCm configuration completed"
}

# Verify installation
verify_installation() {
    log_info "Verifying ROCm installation..."
    
    # Check rocm-smi
    if command -v rocm-smi &> /dev/null; then
        log_info "rocm-smi: OK"
        rocm-smi || true
    else
        log_warn "rocm-smi: Not found"
    fi
    
    # Check rocminfo
    if command -v rocminfo &> /dev/null; then
        log_info "rocminfo: OK"
        rocminfo | head -20 || true
    else
        log_warn "rocminfo: Not found"
    fi
    
    # Check HIP
    if command -v hipcc &> /dev/null; then
        log_info "HIP compiler: OK"
    else
        log_warn "HIP compiler: Not found"
    fi
    
    # Check device access
    if [ -e /dev/kfd ]; then
        log_info "/dev/kfd: OK"
    else
        log_error "/dev/kfd: Not found"
    fi
    
    if [ -e /dev/dri ]; then
        log_info "/dev/dri: OK"
    else
        log_error "/dev/dri: Not found"
    fi
}

# Test Docker access
test_docker() {
    log_info "Testing Docker ROCm access..."
    
    if ! command -v docker &> /dev/null; then
        log_warn "Docker not installed, skipping test"
        return
    fi
    
    # Test with simple container
    docker run --rm --device /dev/kfd --device /dev/dri --group-add video \
        rocm/pytorch:rocm6.0-ubuntu22.04 rocminfo 2>&1 | head -20
    
    if [ $? -eq 0 ]; then
        log_info "Docker ROCm test: PASSED"
    else
        log_warn "Docker ROCm test: FAILED (check permissions)"
    fi
}

# Main
main() {
    log_info "Face Grouper - ROCm Installation Script"
    log_info "======================================="
    
    check_root
    detect_os
    check_gpu
    
    log_info "Starting ROCm installation..."
    install_rocm_ubuntu
    
    configure_groups
    configure_docker
    
    log_info ""
    log_info "Installation completed!"
    log_info "======================="
    
    verify_installation
    
    log_info ""
    log_info "Next steps:"
    log_info "1. Logout and login again for group changes"
    log_info "2. Reboot your system (recommended)"
    log_info "3. Run: docker-compose up -d face-grouper-rocm"
    log_info ""
    log_info "Usage:"
    log_info "  docker run --device /dev/kfd --device /dev/dri --group-add video \\"
    log_info "    -v \$(pwd)/dataset:/app/dataset \\"
    log_info "    -v \$(pwd)/output:/app/output \\"
    log_info "    -v \$(pwd)/models:/app/models \\"
    log_info "    -p 8082:8080 \\"
    log_info "    face-grouper:rocm"
}

main "$@"
