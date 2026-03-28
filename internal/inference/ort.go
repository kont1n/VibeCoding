package inference

import (
	"fmt"
	"runtime"
	"strconv"

	ort "github.com/yalue/onnxruntime_go"
	"github.com/kont1n/face-grouper/internal/inference/provider"
)

var ortInitialized bool
var sessionTuning SessionTuning
var selectedProvider provider.ProviderInfo

// SessionTuning controls ONNX Runtime session-level performance options.
type SessionTuning struct {
	IntraOpThreads int
	InterOpThreads int
}

// ProviderConfig contains configuration for provider selection.
type ProviderConfig struct {
	Preferred     provider.ProviderType
	ForceCPU      bool
	DeviceID      int
	AllowFallback bool
	LogSelection  bool
}

// ProviderInfo is an alias for provider.ProviderInfo for backward compatibility.
type ProviderInfo = provider.ProviderInfo

// SetSessionTuning sets global session tuning used for newly created sessions.
func SetSessionTuning(cfg SessionTuning) {
	sessionTuning = cfg
}

// SelectAndInitializeProvider selects the best available provider and initializes ONNX Runtime.
// This function should be called once at application startup.
func SelectAndInitializeProvider(cfg ProviderConfig, libPath string) error {
	if ortInitialized {
		return nil
	}

	// Select provider
	selectionCfg := provider.SelectionConfig{
		ForceCPU:      cfg.ForceCPU,
		Preferred:     cfg.Preferred,
		AllowFallback: cfg.AllowFallback,
		DeviceID:      cfg.DeviceID,
	}

	selected, err := provider.SelectProvider(selectionCfg)
	if err != nil {
		return fmt.Errorf("select provider: %w", err)
	}

	selectedProvider = selected

	// Initialize ONNX Runtime
	if libPath == "" {
		switch runtime.GOOS {
		case "windows":
			libPath = "onnxruntime.dll"
		case "darwin":
			libPath = "libonnxruntime.dylib"
		default:
			libPath = "libonnxruntime.so"
		}
	}
	ort.SetSharedLibraryPath(libPath)
	if err := ort.InitializeEnvironment(); err != nil {
		return fmt.Errorf("onnxruntime init: %w", err)
	}
	ortInitialized = true

	return nil
}

// GetSelectedProvider returns the currently selected provider.
func GetSelectedProvider() provider.ProviderInfo {
	return selectedProvider
}

// DestroyORT releases ONNX Runtime resources. Call at shutdown.
func DestroyORT() {
	if ortInitialized {
		ort.DestroyEnvironment()
		ortInitialized = false
	}
}

// SessionOptions builds an ort.SessionOptions with the selected provider.
func SessionOptions(cfg ProviderConfig) (*ort.SessionOptions, error) {
	opts, err := ort.NewSessionOptions()
	if err != nil {
		return nil, fmt.Errorf("create session options: %w", err)
	}

	if err := opts.SetGraphOptimizationLevel(ort.GraphOptimizationLevelEnableAll); err != nil {
		opts.Destroy()
		return nil, fmt.Errorf("set graph optimization level: %w", err)
	}
	if err := opts.SetExecutionMode(ort.ExecutionModeParallel); err != nil {
		opts.Destroy()
		return nil, fmt.Errorf("set execution mode: %w", err)
	}
	if sessionTuning.IntraOpThreads > 0 {
		if err := opts.SetIntraOpNumThreads(sessionTuning.IntraOpThreads); err != nil {
			opts.Destroy()
			return nil, fmt.Errorf("set intra-op threads: %w", err)
		}
	}
	if sessionTuning.InterOpThreads > 0 {
		if err := opts.SetInterOpNumThreads(sessionTuning.InterOpThreads); err != nil {
			opts.Destroy()
			return nil, fmt.Errorf("set inter-op threads: %w", err)
		}
	}

	// Add execution provider based on selection
	providerType := cfg.Preferred
	if cfg.ForceCPU {
		providerType = provider.ProviderCPU
	}

	switch providerType {
	case provider.ProviderCUDA:
		cudaOpts, err := ort.NewCUDAProviderOptions()
		if err != nil {
			opts.Destroy()
			return nil, fmt.Errorf("create CUDA provider options: %w", err)
		}
		if err := cudaOpts.Update(map[string]string{
			"do_copy_in_default_stream":    "1",
			"cudnn_conv_use_max_workspace": "1",
			"device_id":                    strconv.Itoa(cfg.DeviceID),
		}); err != nil {
			cudaOpts.Destroy()
			opts.Destroy()
			return nil, fmt.Errorf("configure CUDA provider options: %w", err)
		}
		if err := opts.AppendExecutionProviderCUDA(cudaOpts); err != nil {
			// Fallback to CPU if CUDA fails and allowed
			if cfg.AllowFallback {
				cudaOpts.Destroy()
				// Continue without GPU - already logged warning
			} else {
				cudaOpts.Destroy()
				opts.Destroy()
				return nil, fmt.Errorf("append CUDA provider: %w", err)
			}
		} else {
			cudaOpts.Destroy()
		}

	case provider.ProviderROCm:
		// ROCm support depends on onnxruntime_go version
		// For now, fallback to CPU
		if !cfg.AllowFallback {
			return nil, fmt.Errorf("ROCm provider not available in this build")
		}
		// Continue with CPU fallback

	case provider.ProviderDirectML:
		// DirectML support depends on onnxruntime_go version
		// For now, fallback to CPU
		if !cfg.AllowFallback {
			return nil, fmt.Errorf("DirectML provider not available in this build")
		}
		// Continue with CPU fallback

	case provider.ProviderCoreML:
		// CoreML is typically auto-selected on macOS
		// No special configuration needed
		break
	}

	return opts, nil
}
