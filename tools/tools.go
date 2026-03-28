//go:build tools
// +build tools

package tools

// This file tracks development tool dependencies for the project.
// See: https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module

import (
	// Linters
	_ "github.com/golangci/golangci-lint/v2/cmd/golangci-lint"
	
	// Code generation
	_ "github.com/vektra/mockery/v2"
	_ "github.com/bufbuild/buf/cmd/buf"
	
	// gRPC tools
	_ "github.com/fullstorydev/grpcurl/cmd/grpcurl"
	
	// Formatters
	_ "mvdan.cc/gofumpt"
	_ "github.com/daixiang0/gci"
	
	// Testing
	_ "gotest.tools/gotestsum"
	
	// Security scanning
	_ "github.com/securego/gosec/v2/cmd/gosec"
)
