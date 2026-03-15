package stream

import (
	"github.com/0xJacky/Nginx-UI/internal/helper"
)

// Duplicate duplicates a site by copying the file
func Duplicate(src, dst string) (err error) {
	src, err = ResolveAvailablePath(src)
	if err != nil {
		return err
	}

	dst, err = ResolveAvailablePath(dst)
	if err != nil {
		return err
	}

	if helper.FileExists(dst) {
		return ErrDstFileExists
	}

	_, err = helper.CopyFile(src, dst)
	if err != nil {
		return
	}

	return
}
