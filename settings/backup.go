package settings

// Backup contains configuration settings for backup operations.
// This structure defines security constraints and access permissions for backup functionality.
type Backup struct {
	// GrantedAccessPath defines the list of directory paths that are allowed for backup operations.
	// All backup source paths and storage destination paths must be within one of these directories.
	// This security measure prevents unauthorized access to sensitive system directories.
	//
	// Examples:
	//   - "/tmp" - Allow backups in temporary directory
	//   - "/var/backups" - Allow backups in system backup directory
	//   - "/home/user/backups" - Allow backups in user's backup directory
	//
	// Note: Paths are checked using prefix matching, so "/tmp" allows "/tmp/backup" but not "/tmpfoo"
	GrantedAccessPath []string `json:"granted_access_path" ini:",,allowshadow"`
}

// BackupSettings is the global configuration instance for backup operations.
// This variable holds the current backup security settings and access permissions.
//
// Default configuration:
//   - GrantedAccessPath: Empty list (no paths allowed by default for security)
//
// To enable backup functionality, administrators must explicitly configure allowed paths
// through the settings interface or configuration file.
var BackupSettings = &Backup{
	GrantedAccessPath: []string{
		// Default paths can be added here, but empty for security by default
		// Example: "/tmp", "/var/backups"
	},
}
