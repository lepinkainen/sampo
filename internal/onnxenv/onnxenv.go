package onnxenv

import (
	"os"
	"runtime"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
)

var once sync.Once

// Init sets up the ONNX Runtime shared library and initializes the environment.
// Safe to call multiple times — only the first call has any effect.
func Init() error {
	var initErr error
	once.Do(func() {
		ort.SetSharedLibraryPath(findOrtLibrary())
		initErr = ort.InitializeEnvironment()
	})
	return initErr
}

// Destroy tears down the ONNX Runtime environment.
func Destroy() {
	_ = ort.DestroyEnvironment()
}

// findOrtLibrary returns the platform-specific ONNX Runtime shared library path.
func findOrtLibrary() string {
	if p := os.Getenv("ORT_LIB_PATH"); p != "" {
		return p
	}

	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return "/opt/homebrew/lib/libonnxruntime.dylib"
		}
		return "/usr/local/lib/libonnxruntime.dylib"
	case "linux":
		return "/usr/lib/libonnxruntime.so"
	default:
		return "onnxruntime.dll"
	}
}
