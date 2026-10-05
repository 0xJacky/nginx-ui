# Backup Configuration

The backup section of the Nginx UI configuration controls the security and access permissions for backup operations. This section ensures that backup functionality operates within defined security boundaries while providing flexible storage options.

## Overview

Nginx UI provides comprehensive backup functionality that includes:

- **Manual Backup**: On-demand backup creation through the web interface
- **Automatic Backup**: Scheduled backup tasks with visual cron editor
- **Multiple Backup Types**: Support for Nginx configuration, Nginx UI configuration, or custom directory backups
- **Storage Options**: Local storage and S3-compatible object storage
- **Security**: Encrypted backups with AES encryption for configuration data

## GrantedAccessPath

- Type: `[]string`
- Default: `[]` (empty array)
- Version: `>= v2.0.0`

This is the most critical security setting for backup operations. It defines a list of directory paths that are allowed for backup operations, including both backup source paths and storage destination paths.

### Purpose

The `GrantedAccessPath` setting serves as a security boundary that:

- **Prevents Unauthorized Access**: Restricts backup operations to explicitly authorized directories
- **Protects System Files**: Prevents accidental backup or access to sensitive system directories
- **Enforces Access Control**: Ensures all backup paths are within administrator-defined boundaries
- **Prevents Path Traversal**: Blocks attempts to access directories outside the allowed scope

### Configuration Format

```ini
[backup]
GrantedAccessPath = /var/backups
GrantedAccessPath = /home/user/backups
```

### Path Validation Rules

1. **Prefix Matching**: Paths are validated using prefix matching with proper boundary checking
2. **Path Cleaning**: All paths are normalized to prevent directory traversal attacks (e.g., `../` sequences)
3. **Exact Boundaries**: `/tmp` allows `/tmp/backup` but not `/tmpfoo` to prevent confusion
4. **Empty Default**: By default, no custom directory backup operations are allowed for maximum security

### Security Considerations

- **Default Security**: The default empty configuration ensures no custom directory backup operations are allowed until explicitly configured
- **Explicit Configuration**: Administrators must consciously define allowed paths
- **Regular Review**: Periodically review and update allowed paths based on operational needs
- **Minimal Permissions**: Only grant access to directories that genuinely need backup functionality

## Backup Types

### Configuration Backups

When backing up Nginx or Nginx UI configurations:

- **Encryption**: All configuration backups are automatically encrypted using AES encryption
- **Key Management**: Encryption keys are generated automatically and saved alongside backup files
- **Integrity Verification**: SHA-256 hashes ensure backup integrity
- **Metadata**: Version information and timestamps are included for restoration context

### Custom Directory Backups

For custom directory backups:

- **No Encryption**: Custom directory backups are stored as standard ZIP files without encryption
- **Path Validation**: Source directories must be within `GrantedAccessPath` boundaries
- **Flexible Content**: Can backup any directory structure within allowed paths
- **Output Exclusion**: If the destination is inside the source directory, the
  archive being created is excluded from its own contents. Existing backup files
  remain included; choose a separate storage directory to avoid archiving them.

## Storage Configuration

### Local Storage

- **Path Validation**: Storage paths must be within `GrantedAccessPath` boundaries
- **Directory Creation**: Storage directories are created automatically if they don't exist
- **Permissions**: Backup files are created with secure permissions (0600)

### S3 Storage

For S3-compatible object storage:

- **Required Fields**: Bucket name, access key ID, and secret access key are mandatory
- **Optional Fields**: Endpoint URL and region can be configured for custom S3 providers

## Automatic Backup Scheduling

### Visual Cron Editor

Automatic backups use a visual cron editor interface that allows you to:

- **Select Frequency**: Choose from daily, weekly, monthly, or custom schedules
- **Set Time**: Pick specific hours and minutes for backup execution
- **Preview Schedule**: View human-readable descriptions of the backup schedule

### Task Management

- **Status Tracking**: Each backup task tracks execution status (pending, success, failed)
- **Error Logging**: Failed backups include detailed error messages for troubleshooting

### Backup Retention

By default, every run of an automatic backup task adds a new backup file and old files are never deleted. Set **Backups to Keep** on a task to limit how many backups it retains:

- **Default**: `0` keeps all backups
- **Limit**: A positive number keeps only that many of the newest backups after each successful run and deletes the older ones. A backup that has an encryption key file is deleted together with its `.key` file
- **Storage**: Works for both local and S3 storage
- **Scope**: Only files that follow the task's naming scheme (`<name>_<timestamp>.zip`, or `custom_dir_<name>_<timestamp>.zip` for custom directory backups) directly inside the storage path are considered. Other files are left alone. Characters that are not allowed in file names are replaced with `_` in `<name>`
- **Shared names**: If another task writes backups with the same file names to the same storage path (for example `web prod` and `web/prod`, or a custom directory task `web` and a task named `custom_dir_web`), their backups cannot be told apart. The task then skips the cleanup and sends a warning notification until one of the tasks is renamed or moved to its own storage path
- **Renamed tasks**: Backups written before a task was renamed or moved to another storage path no longer match it and are not deleted. Remove them by hand if they are no longer needed
- **Versioned S3 buckets**: In a bucket with versioning enabled, deleting an old backup only adds a delete marker, so the storage is freed by the bucket's lifecycle rules rather than by the cleanup
- **Failures**: If an old backup cannot be deleted, a warning is logged and the backup task itself still succeeds

This configuration enables backup operations while maintaining strict security boundaries, ensuring that backup functionality cannot be misused to access unauthorized system areas.
