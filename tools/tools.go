//go:build tools
// +build tools

package tools

// This file tracks development tool dependencies for the project.
// See: https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module

import (
	// Code generation
	_ "github.com/bufbuild/buf/cmd/buf"
	// Formatters
	_ "github.com/daixiang0/gci"
	// gRPC tools
	_ "github.com/fullstorydev/grpcurl/cmd/grpcurl"
	// Linters
	_ "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
	// Security scanning
	_ "github.com/securego/gosec/v2/cmd/gosec"
	_ "github.com/vektra/mockery/v2"
	// Testing
	_ "gotest.tools/gotestsum"
	_ "mvdan.cc/gofumpt"
)
