#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="$ROOT_DIR/output/benchmarks"
STAMP="$(date +%Y%m%d-%H%M%S)"
OUT_FILE="$OUT_DIR/extraction-bench-$STAMP.txt"

mkdir -p "$OUT_DIR"

echo "Running extraction benchmarks..."
echo "Output file: $OUT_FILE"

cd "$ROOT_DIR"
go test -bench Benchmark -benchmem ./internal/service/extraction | tee "$OUT_FILE"

echo
echo "Done. Baseline docs: doc/perf.md"
echo "Saved: $OUT_FILE"
