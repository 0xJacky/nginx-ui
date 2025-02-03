package upgrader

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/pkg/errors"
)

type RuntimeInfo struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	ExPath string `json:"ex_path"`
}

type CurVersion struct {
	Version    string `json:"version"`
	BuildID    int    `json:"build_id"`
	TotalBuild int    `json:"total_build"`
}

func GetRuntimeInfo() (r RuntimeInfo, err error) {
	ex, err := os.Executable()
	if err != nil {
		err = errors.Wrap(err, "service.GetRuntimeInfo os.Executable() err")
		return
	}
	realPath, err := filepath.EvalSymlinks(ex)
	if err != nil {
		err = errors.Wrap(err, "service.GetRuntimeInfo filepath.EvalSymlinks() err")
		return
	}

	r = RuntimeInfo{
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		ExPath: realPath,
	}

	return
}
