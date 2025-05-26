package backup

import (
	"github.com/uozi-tech/cosy"
)

var (
	errScope = cosy.NewErrorScope("backup")

	// Backup errors
	ErrCreateTempDir     = errScope.New(4002, "Failed to create temporary directory")
	ErrCreateTempSubDir  = errScope.New(4003, "Failed to create temporary subdirectory")
	ErrBackupNginxUI     = errScope.New(4004, "Failed to backup Nginx UI files: {0}")
	ErrBackupNginx       = errScope.New(4005, "Failed to backup Nginx config files: {0}")
	ErrCreateHashFile    = errScope.New(4006, "Failed to create hash info file: {0}")
	ErrEncryptNginxUIDir = errScope.New(4007, "Failed to encrypt Nginx UI directory: {0}")
	ErrEncryptNginxDir   = errScope.New(4008, "Failed to encrypt Nginx directory: {0}")
	ErrCreateZipArchive  = errScope.New(4009, "Failed to create zip archive: {0}")
	ErrGenerateAESKey    = errScope.New(4011, "Failed to generate AES key: {0}")
	ErrGenerateIV        = errScope.New(4012, "Failed to generate initialization vector: {0}")
	ErrCreateBackupFile  = errScope.New(4013, "Failed to create backup file: {0}")
	ErrCleanupTempDir    = errScope.New(4014, "Failed to cleanup temporary directory: {0}")

	// Config and file errors
	ErrConfigPathEmpty     = errScope.New(4101, "Config path is empty")
	ErrCopyConfigFile      = errScope.New(4102, "Failed to copy config file: {0}")
	ErrCopyDBDir           = errScope.New(4103, "Failed to copy database directory: {0}")
	ErrCopyDBFile          = errScope.New(4104, "Failed to copy database file: {0}")
	ErrCalculateHash       = errScope.New(4105, "Failed to calculate hash: {0}")
	ErrNginxConfigDirEmpty = errScope.New(4106, "Nginx config directory is not set")
	ErrCopyNginxConfigDir  = errScope.New(4107, "Failed to copy Nginx config directory: {0}")
	ErrReadSymlink         = errScope.New(4108, "Failed to read symlink: {0}")

	// Encryption and decryption errors
	ErrReadFile           = errScope.New(4201, "Failed to read file: {0}")
	ErrEncryptFile        = errScope.New(4202, "Failed to encrypt file: {0}")
	ErrWriteEncryptedFile = errScope.New(4203, "Failed to write encrypted file: {0}")
	ErrEncryptData        = errScope.New(4204, "Failed to encrypt data: {0}")
	ErrDecryptData        = errScope.New(4205, "Failed to decrypt data: {0}")
	ErrInvalidPadding     = errScope.New(4206, "Invalid padding in decrypted data")

	// Zip file errors
	ErrCreateZipFile   = errScope.New(4301, "Failed to create zip file: {0}")
	ErrCreateZipEntry  = errScope.New(4302, "Failed to create zip entry: {0}")
	ErrOpenSourceFile  = errScope.New(4303, "Failed to open source file: {0}")
	ErrCreateZipHeader = errScope.New(4304, "Failed to create zip header: {0}")
	ErrCopyContent     = errScope.New(4305, "Failed to copy file content: {0}")
	ErrWriteZipBuffer  = errScope.New(4306, "Failed to write to zip buffer: {0}")

	// Restore errors
	ErrCreateRestoreDir     = errScope.New(4501, "Failed to create restore directory: {0}")
	ErrExtractArchive       = errScope.New(4505, "Failed to extract archive: {0}")
	ErrDecryptNginxUIDir    = errScope.New(4506, "Failed to decrypt Nginx UI directory: {0}")
	ErrDecryptNginxDir      = errScope.New(4507, "Failed to decrypt Nginx directory: {0}")
	ErrVerifyHashes         = errScope.New(4508, "Failed to verify hashes: {0}")
	ErrRestoreNginxConfigs  = errScope.New(4509, "Failed to restore Nginx configs: {0}")
	ErrRestoreNginxUIFiles  = errScope.New(4510, "Failed to restore Nginx UI files: {0}")
	ErrBackupFileNotFound   = errScope.New(4511, "Backup file not found: {0}")
	ErrInvalidSecurityToken = errScope.New(4512, "Invalid security token format")
	ErrInvalidAESKey        = errScope.New(4513, "Invalid AES key format: {0}")
	ErrInvalidAESIV         = errScope.New(4514, "Invalid AES IV format: {0}")

	// Zip extraction errors
	ErrOpenZipFile     = errScope.New(4601, "Failed to open zip file: {0}")
	ErrCreateDir       = errScope.New(4602, "Failed to create directory: {0}")
	ErrCreateParentDir = errScope.New(4603, "Failed to create parent directory: {0}")
	ErrCreateFile      = errScope.New(4604, "Failed to create file: {0}")
	ErrOpenZipEntry    = errScope.New(4605, "Failed to open zip entry: {0}")
	ErrCreateSymlink   = errScope.New(4606, "Failed to create symbolic link: {0}")
	ErrInvalidFilePath = errScope.New(4607, "Invalid file path: {0}")
	ErrEvalSymlinks    = errScope.New(4608, "Failed to evaluate symbolic links: {0}")

	// Decryption errors
	ErrReadEncryptedFile  = errScope.New(4701, "Failed to read encrypted file: {0}")
	ErrDecryptFile        = errScope.New(4702, "Failed to decrypt file: {0}")
	ErrWriteDecryptedFile = errScope.New(4703, "Failed to write decrypted file: {0}")

	// Hash verification errors
	ErrReadHashFile       = errScope.New(4801, "Failed to read hash info file: {0}")
	ErrCalculateUIHash    = errScope.New(4802, "Failed to calculate Nginx UI hash: {0}")
	ErrCalculateNginxHash = errScope.New(4803, "Failed to calculate Nginx hash: {0}")
	ErrHashMismatch       = errScope.New(4804, "Hash verification failed: file integrity compromised")

	// Auto backup errors
	ErrAutoBackupPathNotAllowed        = errScope.New(4901, "Backup path not in granted access paths: {0}")
	ErrAutoBackupStoragePathNotAllowed = errScope.New(4902, "Storage path not in granted access paths: {0}")
	ErrAutoBackupPathRequired          = errScope.New(4903, "Backup path is required for custom directory backup")
	ErrAutoBackupS3ConfigIncomplete    = errScope.New(4904, "S3 configuration is incomplete: missing {0}")
	ErrAutoBackupUnsupportedType       = errScope.New(4905, "Unsupported backup type: {0}")
	ErrAutoBackupCreateDir             = errScope.New(4906, "Failed to create backup directory: {0}")
	ErrAutoBackupWriteFile             = errScope.New(4907, "Failed to write backup file: {0}")
	ErrAutoBackupWriteKeyFile          = errScope.New(4908, "Failed to write security key file: {0}")
	ErrAutoBackupS3Upload              = errScope.New(4909, "S3 upload failed: {0}")
	ErrAutoBackupS3Connection          = errScope.New(4920, "S3 connection test failed: {0}")
	ErrAutoBackupS3BucketAccess        = errScope.New(4921, "S3 bucket access denied: {0}")
	ErrAutoBackupS3InvalidCredentials  = errScope.New(4922, "S3 credentials are invalid: {0}")
	ErrAutoBackupS3InvalidEndpoint     = errScope.New(4923, "S3 endpoint is invalid: {0}")

	// Path validation errors
	ErrInvalidPath            = errScope.New(4910, "Invalid path: {0}")
	ErrPathNotInGrantedAccess = errScope.New(4911, "Path not in granted access paths: {0}")
	ErrBackupPathNotExist     = errScope.New(4912, "Backup path does not exist: {0}")
	ErrBackupPathAccess       = errScope.New(4913, "Cannot access backup path {0}: {1}")
	ErrBackupPathNotDirectory = errScope.New(4914, "Backup path is not a directory: {0}")
	ErrCreateStorageDir       = errScope.New(4915, "Failed to create storage directory {0}: {1}")
	ErrStoragePathAccess      = errScope.New(4916, "Cannot access storage path {0}: {1}")
)
