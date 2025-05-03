package version

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/pkg/errors"
)

type RuntimeInfo struct {
	OS         string `json:"os"`
	Arch       string `json:"arch"`
	ExPath     string `json:"ex_path"`
	CurVersion *Info  `json:"cur_version"`
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
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		ExPath:     realPath,
		CurVersion: GetVersionInfo(),
	}

	return
}
