package geolite

import "github.com/uozi-tech/cosy"

var (
	e                           = cosy.NewErrorScope("geolite")
	ErrDownloadFailed           = e.New(60000, "failed to download GeoLite2 database: {0}")
	ErrDecompressionFailed      = e.New(60001, "failed to decompress GeoLite2 database: {0}")
	ErrDatabaseNotFound         = e.New(60002, "GeoLite2 database not found at {0}")
	ErrFailedToGetFileSize      = e.New(60003, "failed to get file size: {0}")
	ErrFailedToCreateFile       = e.New(60004, "failed to create file: {0}")
	ErrFailedToSaveFile         = e.New(60005, "failed to save downloaded file: {0}")
	ErrFailedToOpenFile         = e.New(60006, "failed to open file: {0}")
	ErrFailedToCreateXZReader   = e.New(60007, "failed to create xz reader: {0}")
	ErrFailedToWriteData        = e.New(60008, "failed to write decompressed data: {0}")
	ErrFailedToReadData         = e.New(60009, "failed to read compressed data: {0}")
	ErrFailedToDeleteCompressed = e.New(60010, "decompression succeeded but failed to delete compressed file: {0}")
)
