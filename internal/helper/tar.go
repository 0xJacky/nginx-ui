package helper

import (
	"archive/tar"
	"compress/gzip"
	"github.com/pkg/errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func UnTar(dst, src string) (err error) {
	fr, err := os.Open(src)
	if err != nil {
		err = errors.Wrap(err, "unTar open src error")
		return
	}
	defer fr.Close()

	gr, err := gzip.NewReader(fr)
	if err != nil {
		err = errors.Wrap(err, "unTar gzip.NewReader error")
		return
	}
	defer gr.Close()

	tr := tar.NewReader(gr)

	for {
		err = func() (err error) {
			hdr, err := tr.Next()

			switch {
			case err == io.EOF:
				return
			case err != nil:
				return errors.Wrap(err, "unTar tr.Next() error")
			case hdr == nil:
				return
			case strings.Contains(hdr.Name, ".."):
				return
			}

			dstFileDir := filepath.Join(dst, hdr.Name)

			switch hdr.Typeflag {
			case tar.TypeDir:
				if b := ExistDir(dstFileDir); !b {
					if err = os.MkdirAll(dstFileDir, 0755); err != nil {
						return errors.Wrap(err, "unTar os.MkdirAll error")
					}
				}
			case tar.TypeReg:
				file, err := os.OpenFile(dstFileDir, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, os.ModePerm)
				if err != nil {
					return errors.Wrap(err, "unTar os.OpenFile error")
				}
				defer file.Close()
				_, err = file.Seek(0, io.SeekEnd)
				if err != nil {
					return errors.Wrap(err, "unTar file.Truncate(0) error")
				}
				_, err = io.Copy(file, tr)
				if err != nil {
					return errors.Wrap(err, "unTar io.Copy error")
				}
			}
			return
		}()
		if err == io.EOF {
			break
		}
	}
	if err == io.EOF {
		err = nil
	}
	return
}

func ExistDir(dirname string) bool {
	fi, err := os.Stat(dirname)
	return (err == nil || os.IsExist(err)) && fi.IsDir()
}
