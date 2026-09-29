package cert

import (
	"archive/zip"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/uozi-tech/cosy"
)

const (
	// MaxCertificateArchiveSize caps the uploaded zip itself.
	MaxCertificateArchiveSize = 5 << 20
	// maxCertificateArchiveEntrySize caps a single decompressed entry; a PEM
	// bundle is a few KB, so anything larger is not a certificate file.
	maxCertificateArchiveEntrySize = 1 << 20
	// maxCertificateArchiveTotalSize caps everything decompressed from one
	// archive so a zip bomb cannot exhaust memory.
	maxCertificateArchiveTotalSize = 16 << 20
	maxCertificateArchiveEntries   = 256
)

// ArchiveCertificate is the certificate pair extracted from an uploaded zip.
type ArchiveCertificate struct {
	Name                  string `json:"name"`
	SSLCertificate        string `json:"ssl_certificate"`
	SSLCertificateKey     string `json:"ssl_certificate_key"`
	CertificateFileName   string `json:"certificate_file_name"`
	PrivateKeyFileName    string `json:"private_key_file_name"`
	ChainCompletedFromZip bool   `json:"chain_completed_from_zip"`
}

type archiveCertificateFile struct {
	name  string
	certs []*x509.Certificate
}

type archivePrivateKeyFile struct {
	name string
	pem  []byte
}

// ParseCertificateArchive extracts a matching certificate and private key
// from a zip, as downloaded from a CA or cloud console (for example a
// "nginx" bundle with xxx_bundle.crt and xxx.key, or a certbot directory
// with fullchain.pem and privkey.pem).
//
// When several certificate files match the key, the one carrying the longest
// chain wins. When the winner only holds the leaf, intermediates found in
// other files of the archive (such as a separate ca-bundle) are appended so
// the result is a full chain nginx can serve.
func ParseCertificateArchive(data []byte) (*ArchiveCertificate, error) {
	if len(data) > MaxCertificateArchiveSize {
		return nil, ErrCertificateArchiveTooLarge
	}

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, cosy.WrapErrorWithParams(ErrInvalidCertificateArchive, err.Error())
	}
	if len(reader.File) > maxCertificateArchiveEntries {
		return nil, ErrCertificateArchiveTooLarge
	}

	var (
		certFiles []archiveCertificateFile
		keyFiles  []archivePrivateKeyFile
		total     int64
	)
	for _, file := range reader.File {
		if skipArchiveEntry(file) {
			continue
		}

		content, readErr := readArchiveEntry(file, &total)
		if readErr != nil {
			return nil, readErr
		}

		certs, keyPEM := parseArchiveEntry(content)
		if len(certs) > 0 {
			certFiles = append(certFiles, archiveCertificateFile{name: file.Name, certs: certs})
		}
		if keyPEM != nil {
			keyFiles = append(keyFiles, archivePrivateKeyFile{name: file.Name, pem: keyPEM})
		}
	}

	if len(certFiles) == 0 {
		return nil, ErrCertificateArchiveNoCertificate
	}
	if len(keyFiles) == 0 {
		return nil, ErrCertificateArchiveNoPrivateKey
	}

	sortArchiveCertificateFiles(certFiles)
	sortArchivePrivateKeyFiles(keyFiles)

	for _, certFile := range certFiles {
		leafPEM := encodeCertificates(certFile.certs[:1])
		for _, keyFile := range keyFiles {
			if _, pairErr := tls.X509KeyPair(leafPEM, keyFile.pem); pairErr != nil {
				continue
			}

			chain, completed := completeCertificateChain(certFile.certs, certFiles)
			return &ArchiveCertificate{
				Name:                  certificateSubjectName(chain[0]),
				SSLCertificate:        string(encodeCertificates(chain)),
				SSLCertificateKey:     string(keyFile.pem),
				CertificateFileName:   certFile.name,
				PrivateKeyFileName:    keyFile.name,
				ChainCompletedFromZip: completed,
			}, nil
		}
	}

	return nil, ErrCertificateArchiveKeyMismatch
}

func skipArchiveEntry(file *zip.File) bool {
	if file.FileInfo().IsDir() {
		return true
	}
	name := path.Clean(strings.ReplaceAll(file.Name, "\\", "/"))
	if strings.HasPrefix(name, "__MACOSX/") || strings.HasPrefix(path.Base(name), "._") {
		return true
	}
	// A private key or PEM bundle never comes close to this size; skipping
	// early avoids decompressing unrelated payloads such as a .jks keystore.
	return file.UncompressedSize64 > maxCertificateArchiveEntrySize
}

func readArchiveEntry(file *zip.File, total *int64) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, cosy.WrapErrorWithParams(ErrInvalidCertificateArchive, err.Error())
	}
	defer rc.Close()

	// The header's size is attacker-controlled, so bound the actual read too.
	content, err := io.ReadAll(io.LimitReader(rc, maxCertificateArchiveEntrySize+1))
	if err != nil {
		return nil, cosy.WrapErrorWithParams(ErrInvalidCertificateArchive, err.Error())
	}
	if len(content) > maxCertificateArchiveEntrySize {
		return nil, ErrCertificateArchiveTooLarge
	}

	*total += int64(len(content))
	if *total > maxCertificateArchiveTotalSize {
		return nil, ErrCertificateArchiveTooLarge
	}
	return content, nil
}

// parseArchiveEntry returns every certificate in the entry and the first
// private key, re-encoded as its own PEM block. A single file may hold both,
// and a DER-encoded .cer is accepted as a lone certificate.
func parseArchiveEntry(content []byte) ([]*x509.Certificate, []byte) {
	var (
		certs  []*x509.Certificate
		keyPEM []byte
		found  bool
	)

	rest := content
	for {
		block, remaining := pem.Decode(rest)
		if block == nil {
			break
		}
		found = true
		rest = remaining

		switch {
		case block.Type == "CERTIFICATE":
			if parsed, err := x509.ParseCertificate(block.Bytes); err == nil {
				certs = append(certs, parsed)
			}
		case keyPEM == nil && strings.HasSuffix(block.Type, "PRIVATE KEY"):
			encoded := pem.EncodeToMemory(block)
			if IsPrivateKey(string(encoded)) {
				keyPEM = encoded
			}
		}
	}

	if !found {
		if parsed, err := x509.ParseCertificate(content); err == nil {
			certs = append(certs, parsed)
		}
	}

	return certs, keyPEM
}

func sortArchiveCertificateFiles(files []archiveCertificateFile) {
	sort.SliceStable(files, func(i, j int) bool {
		if len(files[i].certs) != len(files[j].certs) {
			return len(files[i].certs) > len(files[j].certs)
		}
		left := certificateCandidateRank("", path.Base(files[i].name))
		right := certificateCandidateRank("", path.Base(files[j].name))
		if left != right {
			return left < right
		}
		return files[i].name < files[j].name
	})
}

func sortArchivePrivateKeyFiles(files []archivePrivateKeyFile) {
	sort.SliceStable(files, func(i, j int) bool {
		left := privateKeyCandidateRank("", path.Base(files[i].name))
		right := privateKeyCandidateRank("", path.Base(files[j].name))
		if left != right {
			return left < right
		}
		return files[i].name < files[j].name
	})
}

// completeCertificateChain appends intermediates from the other archive files
// when certs stops before a self-signed root. Roots are left out on purpose:
// clients already trust them and nginx should not send them.
func completeCertificateChain(certs []*x509.Certificate, files []archiveCertificateFile) ([]*x509.Certificate, bool) {
	chain := append([]*x509.Certificate(nil), certs...)
	completed := false

	for len(chain) < 10 {
		last := chain[len(chain)-1]
		if isSelfSignedCertificate(last) {
			break
		}

		issuer := findArchiveIssuer(last, chain, files)
		if issuer == nil || isSelfSignedCertificate(issuer) {
			break
		}
		chain = append(chain, issuer)
		completed = true
	}

	return chain, completed
}

func findArchiveIssuer(child *x509.Certificate, chain []*x509.Certificate, files []archiveCertificateFile) *x509.Certificate {
	for _, file := range files {
		for _, candidate := range file.certs {
			if !bytes.Equal(candidate.RawSubject, child.RawIssuer) || containsCertificate(chain, candidate) {
				continue
			}
			if child.CheckSignatureFrom(candidate) == nil {
				return candidate
			}
		}
	}
	return nil
}

func containsCertificate(certs []*x509.Certificate, target *x509.Certificate) bool {
	for _, c := range certs {
		if c.Equal(target) {
			return true
		}
	}
	return false
}

func isSelfSignedCertificate(c *x509.Certificate) bool {
	return bytes.Equal(c.RawSubject, c.RawIssuer) && c.CheckSignatureFrom(c) == nil
}

func encodeCertificates(certs []*x509.Certificate) []byte {
	var buf bytes.Buffer
	for _, c := range certs {
		_ = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})
	}
	return buf.Bytes()
}
