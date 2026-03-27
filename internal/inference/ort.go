package inference

import (
	"fmt"
	"runtime"

	ort "github.com/yalue/onnxruntime_go"
)

var ortInitialized bool

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
func SessionOptions(gpu bool) *ort.SessionOptions {
	opts, err := ort.NewSessionOptions()
	if err != nil {
		return nil
	}
	if gpu {
		cudaOpts, err := ort.NewCUDAProviderOptions()
		if err == nil {
			opts.AppendExecutionProviderCUDA(cudaOpts)
		}
	}
	return opts
}
