package helper

import (
	"net/url"
)

func UnescapeURL(path string) (decodedPath string) {
	decodedPath = path
	for {
		newDecodedPath, decodeErr := url.PathUnescape(decodedPath)
		if decodeErr != nil || newDecodedPath == decodedPath {
			break
		}
		decodedPath = newDecodedPath
	}
	return
}
