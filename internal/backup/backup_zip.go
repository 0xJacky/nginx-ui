package backup

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/uozi-tech/cosy"
)

// backupDatabaseSnapshot reuses one sanitized view for every alias encountered
// during a single backup walk. SQLite must open the configured database path so
// committed WAL contents are included even when an entry is a hard-link alias.
type backupDatabaseSnapshot struct {
	directory     string
	directoryInfo os.FileInfo
	path          string
}

func (snapshot *backupDatabaseSnapshot) cleanup() {
	if snapshot.directory != "" {
		_ = os.RemoveAll(snapshot.directory)
	}
}

func (snapshot *backupDatabaseSnapshot) containsDirectory(info os.FileInfo) bool {
	return snapshot.directoryInfo != nil && info.IsDir() && os.SameFile(info, snapshot.directoryInfo)
}

func (snapshot *backupDatabaseSnapshot) source(path string, info os.FileInfo) (source string, skip bool, err error) {
	database, sidecar, err := applicationDatabaseFile(path, info)
	if err != nil {
		return "", false, err
	}
	if sidecar {
		return "", true, nil
	}
	// ZIPs and directory staging preserve links as links rather than following
	// them. A database link therefore contains no database bytes to sanitize.
	if !database || info.Mode()&os.ModeSymlink != 0 {
		return path, false, nil
	}
	if snapshot.path == "" {
		directory, err := os.MkdirTemp("", "nginx-ui-backup-database-*")
		if err != nil {
			return "", false, err
		}
		snapshot.directory = directory
		snapshot.directoryInfo, err = os.Stat(directory)
		if err != nil {
			return "", false, err
		}
		candidate := filepath.Join(directory, "database.db")
		if err := snapshotBackupDatabase(applicationDatabasePath(), candidate); err != nil {
			return "", false, err
		}
		snapshot.path = candidate
	}
	return snapshot.path, false, nil
}

// createZipArchive creates a zip archive from a directory
func createZipArchive(zipPath, srcDir string) error {
	// Creating an output alias must never truncate the live application DB.
	if info, err := os.Lstat(zipPath); err == nil {
		database, sidecar, err := applicationDatabaseFile(zipPath, info)
		if err != nil {
			return err
		}
		if database || sidecar {
			return fmt.Errorf("backup output %q refers to the application database", zipPath)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	snapshot := &backupDatabaseSnapshot{}
	defer snapshot.cleanup()
	// Create a new zip file
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return cosy.WrapErrorWithParams(ErrCreateZipFile, err.Error())
	}
	defer zipFile.Close()
	outputInfo, err := zipFile.Stat()
	if err != nil {
		return err
	}

	// Create a new zip writer
	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Walk through all files in the source directory
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// A local destination may be inside the source tree. Do not read the
		// archive while writing it, including through a hard-link alias.
		if os.SameFile(info, outputInfo) {
			return nil
		}
		if snapshot.containsDirectory(info) {
			return filepath.SkipDir
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
		sourcePath, skip, err := snapshot.source(path, info)
		if err != nil {
			return err
		}
		if skip {
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
		source, err := os.Open(sourcePath)
		if err != nil {
			return cosy.WrapErrorWithParams(ErrOpenSourceFile, err.Error())
		}
		defer source.Close()

		// Copy to zip
		_, err = io.Copy(writer, source)
		return err
	})

	if err != nil {
		return err
	}
	return zipWriter.Close()
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
	snapshot := &backupDatabaseSnapshot{}
	defer snapshot.cleanup()
	// Create a zip writer that writes to the buffer
	zipWriter := zip.NewWriter(buffer)
	defer zipWriter.Close()

	// Walk through all files in the source directory
	err := filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if snapshot.containsDirectory(info) {
			return filepath.SkipDir
		}

		// Skip the source directory itself
		if path == sourceDir {
			return nil
		}
		sourcePath, skip, err := snapshot.source(path, info)
		if err != nil {
			return err
		}
		if skip {
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
		file, err := os.Open(sourcePath)
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
