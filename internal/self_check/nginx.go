package self_check

import (
	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/0xJacky/Nginx-UI/internal/nginx"
)

// CheckConfigDir checks if the config directory exists
func CheckConfigDir() error {
	dir := nginx.GetConfPath()
	if dir == "" {
		return ErrConfigDirNotExist
	}
	if !helper.FileExists(dir) {
		return ErrConfigDirNotExist
	}
	return nil
}

// CheckConfigEntryFile checks if the config entry file exists
func CheckConfigEntryFile() error {
	dir := nginx.GetConfPath()
	if dir == "" {
		return ErrConfigEntryFileNotExist
	}
	if !helper.FileExists(dir) {
		return ErrConfigEntryFileNotExist
	}
	return nil
}

// CheckPIDPath checks if the PID path exists
func CheckPIDPath() error {
	path := nginx.GetPIDPath()
	if path == "" {
		return ErrPIDPathNotExist
	}
	if !helper.FileExists(path) {
		return ErrPIDPathNotExist
	}
	return nil
}

// CheckSbinPath checks if the sbin path exists
func CheckSbinPath() error {
	path := nginx.GetSbinPath()
	if path == "" {
		return ErrSbinPathNotExist
	}
	if !helper.FileExists(path) {
		return ErrSbinPathNotExist
	}
	return nil
}

// CheckAccessLogPath checks if the access log path exists
func CheckAccessLogPath() error {
	path := nginx.GetAccessLogPath()
	if path == "" {
		return ErrAccessLogPathNotExist
	}
	if !helper.FileExists(path) {
		return ErrAccessLogPathNotExist
	}
	return nil
}

// CheckErrorLogPath checks if the error log path exists
func CheckErrorLogPath() error {
	path := nginx.GetErrorLogPath()
	if path == "" {
		return ErrErrorLogPathNotExist
	}
	if !helper.FileExists(path) {
		return ErrErrorLogPathNotExist
	}
	return nil
}
