package helper

import (
	"crypto/sha512"
	"fmt"
	"github.com/uozi-tech/cosy/logger"
	"io"
	"os"
)

func DigestSHA512(filepath string) (hashString string) {
	file, err := os.Open(filepath)
	if err != nil {
		logger.Error(err)
		return
	}
	defer file.Close()

	hash := sha512.New()

	_, err = io.Copy(hash, file)
	if err != nil {
		logger.Error(err)
		return
	}

	hashValue := hash.Sum(nil)

	hashString = fmt.Sprintf("%x", hashValue)

	return
}
