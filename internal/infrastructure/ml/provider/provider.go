// Package provider provides automatic detection and selection of ONNX Runtime execution providers.
package provider

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ProviderType represents the type of execution provider.
type ProviderType string

const (
	// ProviderCPU is the default CPU execution provider.
	ProviderCPU ProviderType = "cpu"
	// ProviderCUDA is the NVIDIA CUDA execution provider.
	ProviderCUDA ProviderType = "cuda"
	// ProviderROCm is the AMD ROCm execution provider.
	ProviderROCm ProviderType = "rocm"
	// ProviderDirectML is the Microsoft DirectML execution provider (Windows).
	ProviderDirectML ProviderType = "directml"
	// ProviderCoreML is the Apple CoreML execution provider (macOS).
	ProviderCoreML ProviderType = "coreml"
)

// ProviderInfo contains information about an execution provider.
type ProviderInfo struct {
	// Type is the provider type.
	Type ProviderType
	// Name is a human-readable provider name.
	Name string
	// Available indicates whether the provider is available on this system.
	Available bool
	// DeviceID is the device identifier (for multi-GPU systems).
	DeviceID int
	// Error contains the error if provider detection failed.
	Error error
	// Priority is the provider priority (lower is better).
	Priority int
}

// DetectionResult contains the result of provider detection.
type DetectionResult struct {
	// Providers is a list of all detected providers.
	Providers []ProviderInfo
	// Selected is the recommended provider based on priority.
	Selected ProviderInfo
}

// DetectAvailableProviders scans the system for available execution providers.
func DetectAvailableProviders() *DetectionResult {
	result := &DetectionResult{
		Providers: make([]ProviderInfo, 0),
	}

	// Always add CPU as fallback
	result.Providers = append(result.Providers, ProviderInfo{
		Type:      ProviderCPU,
		Name:      "CPU",
		Available: true,
		Priority:  100, // Lowest priority
	})

	// Detect GPU providers
	detectedGPU := detectGPU()
	result.Providers = append(result.Providers, detectedGPU...)

	// Select best provider (lowest priority number)
	bestPriority := 101
	for _, p := range result.Providers {
		if p.Available && p.Priority < bestPriority {
			bestPriority = p.Priority
			result.Selected = p
		}
	}

	// If no GPU found, select CPU
	if !result.Selected.Available || result.Selected.Type == "" {
		for _, p := range result.Providers {
			if p.Type == ProviderCPU {
				result.Selected = p
				break
			}
		}
	}

	return result
}

// detectGPU detects available GPU providers.
func detectGPU() []ProviderInfo {
	var providers []ProviderInfo

	// Try CUDA first (highest priority on Linux/Windows with NVIDIA)
	if cuda := detectCUDA(); cuda.Available {
		providers = append(providers, cuda)
	}

	// Try ROCm (AMD GPUs on Linux)
	if rocm := detectROCm(); rocm.Available {
		providers = append(providers, rocm)
	}

	// Try DirectML (Windows with any GPU)
	if runtime.GOOS == "windows" {
		if directml := detectDirectML(); directml.Available {
			providers = append(providers, directml)
		}
	}

	// Try CoreML (macOS)
	if runtime.GOOS == "darwin" {
		if coreml := detectCoreML(); coreml.Available {
			providers = append(providers, coreml)
		}
	}

	return providers
}

// detectCUDA checks for NVIDIA CUDA availability.
func detectCUDA() ProviderInfo {
	provider := ProviderInfo{
		Type:      ProviderCUDA,
		Name:      "CUDA",
		Available: false,
		Priority:  10, // High priority
	}

	// Check nvidia-smi
	cmd := exec.Command("nvidia-smi")
	if err := cmd.Run(); err != nil {
		provider.Error = fmt.Errorf("nvidia-smi not available: %w", err)
		return provider
	}

	// Check CUDA environment variables
	cudaHome := os.Getenv("CUDA_HOME")
	cudaPath := os.Getenv("CUDA_PATH")
	libraryPath := os.Getenv("LD_LIBRARY_PATH")

	if cudaHome == "" && cudaPath == "" {
		// On Windows, check PATH
		if runtime.GOOS == "windows" {
			path := os.Getenv("PATH")
			if !strings.Contains(path, "CUDA") && !strings.Contains(path, "cuda") {
				provider.Error = fmt.Errorf("CUDA not found in PATH")
				return provider
			}
		} else {
			// On Linux, check LD_LIBRARY_PATH
			if !strings.Contains(libraryPath, "cuda") && !strings.Contains(libraryPath, "nvidia") {
				provider.Error = fmt.Errorf("CUDA libraries not found in LD_LIBRARY_PATH")
				// Don't return - nvidia-smi passed, so CUDA might still work
			}
		}
	}

	// Get device ID if specified
	deviceID := 0
	if deviceIDStr := os.Getenv("CUDA_VISIBLE_DEVICES"); deviceIDStr != "" {
		// Parse first device ID
		if strings.Contains(deviceIDStr, ",") {
			deviceIDStr = strings.Split(deviceIDStr, ",")[0]
		}
		if strings.Contains(deviceIDStr, "-") {
			deviceIDStr = strings.Split(deviceIDStr, "-")[0]
		}
		fmt.Sscanf(deviceIDStr, "%d", &deviceID)
	}

	provider.Available = true
	provider.DeviceID = deviceID
	provider.Name = fmt.Sprintf("CUDA (Device %d)", deviceID)
	return provider
}

// detectROCm checks for AMD ROCm availability.
func detectROCm() ProviderInfo {
	provider := ProviderInfo{
		Type:      ProviderROCm,
		Name:      "ROCm",
		Available: false,
		Priority:  20, // Medium-high priority
	}

	// Check ROCm environment variables
	rocmPath := os.Getenv("ROCM_PATH")
	hipPath := os.Getenv("HIP_PATH")
	hipVisibleDevices := os.Getenv("HIP_VISIBLE_DEVICES")

	// Also check for rocm-smi (ROCm system management interface)
	cmd := exec.Command("rocm-smi")
	if err := cmd.Run(); err != nil {
		// rocm-smi not available, but ROCm might still work
		if rocmPath == "" && hipPath == "" {
			provider.Error = fmt.Errorf("ROCm not detected: ROCM_PATH and HIP_PATH not set, rocm-smi unavailable")
			return provider
		}
	}

	// Check for ROCm libraries
	libPaths := []string{
		filepath.Join(rocmPath, "lib"),
		filepath.Join(hipPath, "lib"),
		"/opt/rocm/lib",
		"/usr/lib/x86_64-linux-gnu",
		"/usr/local/lib",
	}

	libFound := false
	for _, path := range libPaths {
		// Check for shared library
		if _, err := os.Stat(filepath.Join(path, "libonnxruntime_providers_rocm.so")); err == nil {
			libFound = true
			break
		}
		// Check for static library
		if _, err := os.Stat(filepath.Join(path, "libonnxruntime_providers_rocm.a")); err == nil {
			libFound = true
			break
		}
		// Check for ROCm ONNX Runtime
		if _, err := os.Stat(filepath.Join(path, "libonnxruntime.so")); err == nil {
			// Verify it's ROCm build by checking for ROCm symbols
			libFound = true
			break
		}
	}

	if !libFound {
		provider.Error = fmt.Errorf("ROCm libraries not found in standard paths")
		return provider
	}

	// Get device ID
	deviceID := 0
	if hipVisibleDevices != "" {
		fmt.Sscanf(hipVisibleDevices, "%d", &deviceID)
	}

	provider.Available = true
	provider.DeviceID = deviceID
	provider.Name = fmt.Sprintf("ROCm (Device %d)", deviceID)
	return provider
}

// detectDirectML checks for Microsoft DirectML availability (Windows).
func detectDirectML() ProviderInfo {
	provider := ProviderInfo{
		Type:      ProviderDirectML,
		Name:      "DirectML",
		Available: false,
		Priority:  30, // Medium priority
	}

	if runtime.GOOS != "windows" {
		provider.Error = fmt.Errorf("DirectML only available on Windows")
		return provider
	}

	// DirectML is part of Windows 10+, check if we can load it
	// This is a basic check - in production you might want to verify the DLL exists
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = `C:\Windows`
	}

	directMLPath := filepath.Join(systemRoot, "System32", "DirectML.dll")
	if _, err := os.Stat(directMLPath); err == nil {
		provider.Available = true
		provider.Name = "DirectML (Windows)"
	} else {
		provider.Error = fmt.Errorf("DirectML.dll not found")
	}

	return provider
}

// detectCoreML checks for Apple CoreML availability (macOS).
func detectCoreML() ProviderInfo {
	provider := ProviderInfo{
		Type:      ProviderCoreML,
		Name:      "CoreML",
		Available: false,
		Priority:  15, // High priority on macOS
	}

	if runtime.GOOS != "darwin" {
		provider.Error = fmt.Errorf("CoreML only available on macOS")
		return provider
	}

	// CoreML is available on macOS 10.13+
	// For now, assume it's available on macOS
	provider.Available = true
	provider.Name = "CoreML (macOS)"
	return provider
}

// ParseProviderType parses a string into a ProviderType.
func ParseProviderType(s string) ProviderType {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "cuda", "gpu", "nvidia":
		return ProviderCUDA
	case "rocm", "amd":
		return ProviderROCm
	case "directml", "dml":
		return ProviderDirectML
	case "coreml", "apple":
		return ProviderCoreML
	case "cpu":
		return ProviderCPU
	default:
		return ""
	}
}

// String returns the string representation of a ProviderType.
func (p ProviderType) String() string {
	return string(p)
}
