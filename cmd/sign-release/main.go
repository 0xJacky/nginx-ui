package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"aead.dev/minisign"
	"github.com/0xJacky/Nginx-UI/internal/releasesign"
)

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		fatal("at least one archive path is required")
	}

	privateKeyText := []byte(os.Getenv("MINISIGN_PRIVATE_KEY"))
	if len(privateKeyText) == 0 {
		fatal("MINISIGN_PRIVATE_KEY is required")
	}
	privateKey, err := loadPrivateKey(privateKeyText, os.Getenv("MINISIGN_PASSWORD"))
	if err != nil {
		fatal("load Minisign private key: %v", err)
	}
	if err = requireTrustedSigner(privateKey, releasesign.TrustedPublicKeys()); err != nil {
		fatal("validate release signer: %v", err)
	}

	for _, archivePath := range flag.Args() {
		if err = signArchive(privateKey, archivePath); err != nil {
			fatal("sign %s: %v", archivePath, err)
		}
		fmt.Printf("signed %s with key %016X\n", archivePath, privateKey.ID())
	}
}

func loadPrivateKey(encodedKey []byte, password string) (minisign.PrivateKey, error) {
	if minisign.IsEncrypted(encodedKey) {
		return minisign.DecryptKey(password, encodedKey)
	}
	var privateKey minisign.PrivateKey
	if err := privateKey.UnmarshalText(encodedKey); err != nil {
		return minisign.PrivateKey{}, err
	}
	return privateKey, nil
}

func requireTrustedSigner(privateKey minisign.PrivateKey, encodedKeys []string) error {
	for _, encodedKey := range encodedKeys {
		encodedKey = strings.TrimSpace(encodedKey)
		if encodedKey == "" {
			continue
		}
		var publicKey minisign.PublicKey
		if err := publicKey.UnmarshalText([]byte(encodedKey)); err != nil {
			return err
		}
		if publicKey.ID() == privateKey.ID() {
			return nil
		}
	}
	return fmt.Errorf("private key ID %016X is not in the pinned release keys", privateKey.ID())
}

func signArchive(privateKey minisign.PrivateKey, archivePath string) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()

	reader := minisign.NewReader(archive)
	if _, err = io.Copy(io.Discard, reader); err != nil {
		return err
	}
	return os.WriteFile(archivePath+".minisig", reader.Sign(privateKey), 0o644)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
