package site

import (
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
	"github.com/pkg/errors"
)

// Duplicate duplicates a site by copying the file
func Duplicate(src, dst string) (err error) {
	src = nginx.GetConfPath("sites-available", src)
	dst = nginx.GetConfPath("sites-available", dst)

	if helper.FileExists(dst) {
		return errors.New("file exists")
	}

	_, err = helper.CopyFile(src, dst)
	if err != nil {
		return
	}

	return
}
