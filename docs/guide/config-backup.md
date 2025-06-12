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

This configuration enables backup operations while maintaining strict security boundaries, ensuring that backup functionality cannot be misused to access unauthorized system areas.