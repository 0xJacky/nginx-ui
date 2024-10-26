package helper

import "os"

func FileExists(filepath string) bool {
	_, err := os.Stat(filepath)
	if os.IsNotExist(err) {
		return false
	}

	return true
}

func SymbolLinkExists(filepath string) bool {
	_, err := os.Lstat(filepath)
	if os.IsNotExist(err) {
		return false
	}
	return true
}
