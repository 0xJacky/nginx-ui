package stream

import (
	"github.com/0xJacky/Nginx-UI/internal/config"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
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

	destinationExists, err := nginx.Exists(dst)
	if err != nil {
		return err
	}
	if destinationExists {
		return ErrDstFileExists
	}

	content, err := nginx.ReadFile(src)
	if err != nil {
		return err
	}

	err = config.ValidateConfigFileBytes(dst, content)
	if err != nil {
		return err
	}

	err = nginx.WriteFile(dst, content, 0644)
	if err != nil {
		return
	}

	return
}
