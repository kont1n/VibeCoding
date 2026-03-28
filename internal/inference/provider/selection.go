package provider

import (
	"fmt"
	"os"
	"strings"
)

// SelectionConfig contains configuration for provider selection.
type SelectionConfig struct {
	// ForceCPU forces CPU usage regardless of available providers.
	ForceCPU bool
	// Preferred is the preferred provider type.
	Preferred ProviderType
	// AllowFallback allows falling back to CPU if preferred provider is unavailable.
	AllowFallback bool
	// DeviceID is the preferred device ID (for multi-GPU systems).
	DeviceID int
}

// DefaultSelectionConfig returns the default selection configuration.
func DefaultSelectionConfig() SelectionConfig {
	return SelectionConfig{
		ForceCPU:      getEnvBool("FORCE_CPU", false),
		Preferred:     ParseProviderType(getEnv("PROVIDER_PRIORITY", "auto")),
		AllowFallback: true,
		DeviceID:      getEnvInt("GPU_DEVICE_ID", 0),
	}
}

// SelectProvider selects the best available provider based on configuration.
func SelectProvider(cfg SelectionConfig) (ProviderInfo, error) {
	// Force CPU if requested
	if cfg.ForceCPU {
		return ProviderInfo{
			Type:      ProviderCPU,
			Name:      "CPU (forced)",
			Available: true,
			Priority:  0,
		}, nil
	}

	// Detect available providers
	detection := DetectAvailableProviders()

	// If preferred provider is specified, try to use it
	if cfg.Preferred != "" && cfg.Preferred != ProviderCPU {
		for _, p := range detection.Providers {
			if p.Type == cfg.Preferred && p.Available {
				if cfg.DeviceID >= 0 {
					p.DeviceID = cfg.DeviceID
				}
				return p, nil
			}
		}

		// Preferred provider not available
		if !cfg.AllowFallback {
			return ProviderInfo{}, fmt.Errorf("preferred provider %s not available", cfg.Preferred)
		}
	}

	// Use auto selection (best available)
	selected := detection.Selected

	// Override device ID if specified
	if cfg.DeviceID >= 0 && selected.Type != ProviderCPU {
		selected.DeviceID = cfg.DeviceID
	}

	return selected, nil
}

// getEnv gets an environment variable with a default value.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an integer environment variable with a default value.
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var intValue int
		if _, err := fmt.Sscanf(value, "%d", &intValue); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvBool gets a boolean environment variable with a default value.
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		switch strings.ToLower(value) {
		case "true", "1", "yes", "on":
			return true
		case "false", "0", "no", "off":
			return false
		}
	}
	return defaultValue
}

// LogProviderSelection logs the provider selection decision.
func LogProviderSelection(selected ProviderInfo, fallback bool, logFunc func(level string, msg string, keysAndValues ...interface{})) {
	if logFunc == nil {
		return
	}

	if fallback {
		logFunc("warn", "Provider fallback occurred",
			"selected", selected.Name,
			"type", selected.Type,
		)
	} else {
		logFunc("info", "ONNX Runtime provider selected",
			"provider", selected.Name,
			"type", selected.Type,
			"device_id", selected.DeviceID,
		)
	}
}
