#!/bin/bash
# =============================================================================
# Face Grouper - Development Tools Installation Script
# =============================================================================
# Installs all development tools to ./bin directory
# Usage: ./scripts/install-tools.sh
# =============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BIN_DIR="$PROJECT_DIR/bin"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_step() { echo -e "${BLUE}[STEP]${NC} $1"; }

# Create bin directory
mkdir -p "$BIN_DIR"

# Install golangci-lint
install_golangci_lint() {
    log_step "Installing golangci-lint..."
    if [ -f "$BIN_DIR/golangci-lint" ]; then
        log_info "golangci-lint already installed"
    else
        GOBIN="$BIN_DIR" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.1.5
        log_info "golangci-lint installed"
    fi
}

# Install mockery
install_mockery() {
    log_step "Installing mockery..."
    if [ -f "$BIN_DIR/mockery" ]; then
        log_info "mockery already installed"
    else
        GOBIN="$BIN_DIR" go install github.com/vektra/mockery/v2@v2.53.3
        log_info "mockery installed"
    fi
}

# Install buf
install_buf() {
    log_step "Installing buf..."
    if [ -f "$BIN_DIR/buf" ]; then
        log_info "buf already installed"
    else
        GOBIN="$BIN_DIR" go install github.com/bufbuild/buf/cmd/buf@latest
        log_info "buf installed"
    fi
}

# Install grpcurl
install_grpcurl() {
    log_step "Installing grpcurl..."
    if [ -f "$BIN_DIR/grpcurl" ]; then
        log_info "grpcurl already installed"
    else
        GOBIN="$BIN_DIR" go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest
        log_info "grpcurl installed"
    fi
}

# Install gofumpt
install_gofumpt() {
    log_step "Installing gofumpt..."
    if [ -f "$BIN_DIR/gofumpt" ]; then
        log_info "gofumpt already installed"
    else
        GOBIN="$BIN_DIR" go install mvdan.cc/gofumpt@latest
        log_info "gofumpt installed"
    fi
}

# Install gci
install_gci() {
    log_step "Installing gci..."
    if [ -f "$BIN_DIR/gci" ]; then
        log_info "gci already installed"
    else
        GOBIN="$BIN_DIR" go install github.com/daixiang0/gci@latest
        log_info "gci installed"
    fi
}

# Install gotestsum
install_gotestsum() {
    log_step "Installing gotestsum..."
    if [ -f "$BIN_DIR/gotestsum" ]; then
        log_info "gotestsum already installed"
    else
        GOBIN="$BIN_DIR" go install gotest.tools/gotestsum@latest
        log_info "gotestsum installed"
    fi
}

# Install gosec
install_gosec() {
    log_step "Installing gosec..."
    if [ -f "$BIN_DIR/gosec" ]; then
        log_info "gosec already installed"
    else
        GOBIN="$BIN_DIR" go install github.com/securego/gosec/v2/cmd/gosec@latest
        log_info "gosec installed"
    fi
}

# Main
main() {
    log_info "Face Grouper - Development Tools Installation"
    log_info "=============================================="
    log_info "Installing to: $BIN_DIR"
    log_info ""
    
    # Add bin to PATH
    export PATH="$BIN_DIR:$PATH"
    
    # Install core tools
    install_golangci_lint
    install_mockery
    install_gofumpt
    install_gci
    
    # Install optional tools
    log_step "Installing optional tools..."
    install_buf || log_warn "buf installation failed (optional)"
    install_grpcurl || log_warn "grpcurl installation failed (optional)"
    install_gotestsum || log_warn "gotestsum installation failed (optional)"
    install_gosec || log_warn "gosec installation failed (optional)"
    
    # Verify installations
    log_info ""
    log_info "Verifying installations..."
    
    tools=("golangci-lint" "mockery" "gofumpt" "gci")
    for tool in "${tools[@]}"; do
        if [ -f "$BIN_DIR/$tool" ]; then
            echo "  ✅ $tool"
        else
            echo "  ❌ $tool (failed)"
        fi
    done
    
    log_info ""
    log_info "✅ Installation completed!"
    log_info ""
    log_info "Add to your PATH:"
    log_info "  export PATH=\"$BIN_DIR:\$PATH\""
    log_info ""
    log_info "Or add to your ~/.bashrc or ~/.zshrc:"
    log_info "  echo 'export PATH=\"$BIN_DIR:\$PATH\"' >> ~/.bashrc"
}

main "$@"
