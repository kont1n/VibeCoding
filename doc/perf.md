# Performance Benchmarks

This document tracks local benchmark baselines for extraction hot paths.

## Environment

- OS: macOS (darwin arm64)
- CPU: Apple M3 Pro
- Go: use project toolchain from `go.mod`

## Current Baseline

Command:

`go test -bench Benchmark -benchmem ./internal/service/extraction`

Results:

| Benchmark | ns/op | allocs/op | B/op | Custom metric |
| --- | ---: | ---: | ---: | --- |
| `BenchmarkPrepareThumbnailImage-12` | 3009470 | 4 | 3489904 | - |
| `BenchmarkThumbnailWriter_SubmitAndFlush-12` | 5752357 | 552 | 6200767 | - |
| `BenchmarkExtractionThroughputSynthetic-12` | 817766 | 1478 | 545121 | `1850957 files/s` |

## How To Re-run

- Run all extraction benchmarks:
  - `bash scripts/bench.sh`
- Run a specific benchmark:
  - `go test -bench BenchmarkExtractionThroughputSynthetic -benchmem ./internal/service/extraction`

## Notes

- Compare results only on the same machine/profile.
- Close heavy background apps before measuring.
- For reliable trend checks, run at least 3 times.
