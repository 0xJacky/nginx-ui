package helper

import (
	"crypto/sha512"
	"fmt"
	"io"
	"log"
	"os"
)

func DigestSHA512(filepath string) (hashString string) {
	file, err := os.Open(filepath)
	if err != nil {
		log.Println("DigestSHA512 open file error")
		return
	}
	defer file.Close()

	hash := sha512.New()

	_, err = io.Copy(hash, file)
	if err != nil {
		log.Println("DigestSHA512 io.Copy error")
		return
	}

	hashValue := hash.Sum(nil)

	hashString = fmt.Sprintf("%x", hashValue)

	return
}
