package inference

import (
	"fmt"
	"runtime"

	ort "github.com/yalue/onnxruntime_go"
)

var ortInitialized bool
var sessionTuning SessionTuning

// SessionTuning controls ONNX Runtime session-level performance options.
type SessionTuning struct {
	IntraOpThreads int
	InterOpThreads int
}

// SetSessionTuning sets global session tuning used for newly created sessions.
func SetSessionTuning(cfg SessionTuning) {
	sessionTuning = cfg
}

// InitORT loads the ONNX Runtime shared library. Call once at startup.
// If libPath is empty, uses the default library name for the current OS.
func InitORT(libPath string) error {
	if ortInitialized {
		return nil
	}
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

// DestroyORT releases ONNX Runtime resources. Call at shutdown.
func DestroyORT() {
	if ortInitialized {
		ort.DestroyEnvironment()
		ortInitialized = false
	}
}

// SessionOptions builds an ort.SessionOptions with CUDA if requested.
func SessionOptions(gpu bool) (*ort.SessionOptions, error) {
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

	if gpu {
		cudaOpts, err := ort.NewCUDAProviderOptions()
		if err != nil {
			opts.Destroy()
			return nil, fmt.Errorf("create CUDA provider options: %w", err)
		}
		if err := cudaOpts.Update(map[string]string{
			"do_copy_in_default_stream":    "1",
			"cudnn_conv_use_max_workspace": "1",
		}); err != nil {
			cudaOpts.Destroy()
			opts.Destroy()
			return nil, fmt.Errorf("configure CUDA provider options: %w", err)
		}
		if err := opts.AppendExecutionProviderCUDA(cudaOpts); err != nil {
			cudaOpts.Destroy()
			opts.Destroy()
			return nil, fmt.Errorf("append CUDA provider: %w", err)
		}
		cudaOpts.Destroy()
	}
	return opts, nil
}
