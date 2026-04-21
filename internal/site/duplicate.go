package site

import (
	"os"

	"github.com/0xJacky/Nginx-UI/internal/config"
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

	content, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	err = config.ValidateConfigFileBytes(dst, content)
	if err != nil {
		return err
	}

	err = os.WriteFile(dst, content, 0644)
	if err != nil {
		return
	}

	return
}
