# ==============================================================================
# Face Grouper - Multi-stage Dockerfile
# ==============================================================================

# ------------------------------------------------------------------------------
# Stage 1: Builder
# ------------------------------------------------------------------------------
FROM golang:1.25-bookworm AS builder

WORKDIR /app

# Install system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the server binary with optimizations
RUN CGO_ENABLED=1 go build \
    -ldflags="-s -w -X main.version=${VERSION:-dev}" \
    -o /app/server \
    ./cmd/main.go

# ------------------------------------------------------------------------------
# Stage 2: Runtime
# ------------------------------------------------------------------------------
FROM debian:bookworm-slim

# Install runtime dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    libonnxruntime1.16.0 \
    libstdc++6 \
    && rm -rf /var/lib/apt/lists/* \
    && apt-get clean

# Create non-root user for security
RUN groupadd -r appuser && useradd -r -g appuser appuser

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/server .

# Copy models and web directories if they exist
COPY --chown=appuser:appuser models ./models 2>/dev/null || true
COPY --chown=appuser:appuser web ./web 2>/dev/null || true

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Set environment variables
ENV GIN_MODE=release

# Entry point
ENTRYPOINT ["./server"]
