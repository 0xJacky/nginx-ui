package backup

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"

	"github.com/uozi-tech/cosy"
)

// createZipArchive creates a zip archive from a directory
func createZipArchive(zipPath, srcDir string) error {
	// Create a new zip file
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrCreateZipFile, err.Error())
	}
	defer zipFile.Close()

	// Create a new zip writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Walk through all files in the source directory
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get relative path
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		// Skip if it's the source directory itself
		if relPath == "." {
			return nil
		}

		// Check if it's a symlink
		if info.Mode()&os.ModeSymlink != 0 {
			// Get target of symlink
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return cosy.WrapErrorWithParams(ErrReadSymlink, err.Error())
			}

			// Create symlink entry in zip
			header := &zip.FileHeader{
				Name:   relPath,
				Method: zip.Deflate,
			}
			header.SetMode(info.Mode())

			writer, err := zipWriter.CreateHeader(header)
			if err != nil {
				return cosy.WrapErrorWithParams(ErrCreateZipEntry, err.Error())
			}

			// Write link target as content (common way to store symlinks in zip)
			_, err = writer.Write([]byte(linkTarget))
			if err != nil {
				return cosy.WrapErrorWithParams(ErrCopyContent, relPath)
			}

			return nil
		}

		// Create zip header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrCreateZipHeader, err.Error())
		}

		// Set relative path as name
		header.Name = relPath
		if info.IsDir() {
			header.Name += "/"
		}

		// Set compression method
		header.Method = zip.Deflate

		// Create zip entry writer
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrCreateZipEntry, err.Error())
		}

		// Skip if it's a directory
		if info.IsDir() {
			return nil
		}

		// Open source file
		source, err := os.Open(path)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrOpenSourceFile, err.Error())
		}
		defer source.Close()

		// Copy to zip
		_, err = io.Copy(writer, source)
		return err
	})

	return err
}

// createZipArchiveFromFiles creates a zip archive from a list of files
func createZipArchiveFromFiles(zipPath string, files []string) error {
	// Create a new zip file
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrCreateZipFile, err.Error())
	}
	defer zipFile.Close()

	// Create a new zip writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Add each file to the zip
	for _, file := range files {
		// Get file info
		info, err := os.Stat(file)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrOpenSourceFile, err.Error())
		}

		// Create zip header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrCreateZipHeader, err.Error())
		}

		// Set base name as header name
		header.Name = filepath.Base(file)

		// Set compression method
		header.Method = zip.Deflate

		// Create zip entry writer
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrCreateZipEntry, err.Error())
		}

		// Open source file
		source, err := os.Open(file)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrOpenSourceFile, err.Error())
		}
		defer source.Close()

		// Copy to zip
		_, err = io.Copy(writer, source)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrCopyContent, file)
		}
	}

	return nil
}

// calculateFileHash calculates the SHA-256 hash of a file
func calculateFileHash(filePath string) (string, error) {
	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return "", cosy.WrapErrorWithParams(ErrReadFile, filePath)
	}
	defer file.Close()

	// Create hash
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", cosy.WrapErrorWithParams(ErrCalculateHash, err.Error())
	}

	// Return hex hash
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// createZipArchiveToBuffer creates a zip archive of files in the specified directory
// and writes the zip content to the provided buffer
func createZipArchiveToBuffer(buffer *bytes.Buffer, sourceDir string) error {
	// Create a zip writer that writes to the buffer
	zipWriter := zip.NewWriter(buffer)
	defer zipWriter.Close()

	// Walk through all files in the source directory
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the source directory itself
		if path == sourceDir {
			return nil
		}

		// Get the relative path to the source directory
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Check if it's a symlink
		if info.Mode()&os.ModeSymlink != 0 {
			// Get target of symlink
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return cosy.WrapErrorWithParams(ErrReadSymlink, err.Error())
			}

			// Create symlink entry in zip
			header := &zip.FileHeader{
				Name:   relPath,
				Method: zip.Deflate,
			}
			header.SetMode(info.Mode())

			writer, err := zipWriter.CreateHeader(header)
			if err != nil {
				return cosy.WrapErrorWithParams(ErrCreateZipEntry, err.Error())
			}

			// Write link target as content
			_, err = writer.Write([]byte(linkTarget))
			if err != nil {
				return cosy.WrapErrorWithParams(ErrCopyContent, relPath)
			}

			return nil
		}

		// Create a zip header from the file info
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrCreateZipHeader, err.Error())
		}

		// Set the name to be relative to the source directory
		header.Name = relPath

		// Set the compression method
		if !info.IsDir() {
			header.Method = zip.Deflate
		}

		// Create the entry in the zip file
		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrCreateZipEntry, err.Error())
		}

		// If it's a directory, we're done
		if info.IsDir() {
			return nil
		}

		// Open the source file
		file, err := os.Open(path)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrOpenSourceFile, err.Error())
		}
		defer file.Close()

		// Copy the file contents to the zip entry
		_, err = io.Copy(writer, file)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrCopyContent, relPath)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Close the zip writer to ensure all data is written
	return zipWriter.Close()
}
