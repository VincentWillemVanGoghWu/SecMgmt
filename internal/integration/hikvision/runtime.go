package hikvision

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type Runtime struct {
	SDKPath string
}

func NewRuntime(sdkPath string) *Runtime {
	return &Runtime{SDKPath: sdkPath}
}

func (r *Runtime) Validate() error {
	if r.SDKPath == "" {
		return fmt.Errorf("hikvision sdk path is empty")
	}
	return nil
}

func (r *Runtime) EffectiveLibraryDir() string {
	if runtime.GOOS == "windows" {
		libDir := filepath.Join(r.SDKPath, "Lib")
		if _, err := os.Stat(libDir); err == nil {
			return libDir
		}
		return filepath.Join(r.SDKPath, "Library")
	}
	return filepath.Join(r.SDKPath, "Library")
}
