// Auto-generated notification texts
// Extracted from Go source code notification function calls
/* eslint-disable ts/no-explicit-any */

const notifications: Record<string, { title: () => string, content: (args: any) => string }> = {

  // general module notifications
  'Reload Remote Nginx Error': {
    title: () => $gettext('Reload Remote Nginx Error'),
    content: (args: any) => $gettext('Reload Nginx on %{node} failed, response: %{resp}', args, true),
  },
  'Reload Remote Nginx Success': {
    title: () => $gettext('Reload Remote Nginx Success'),
    content: (args: any) => $gettext('Reload Nginx on %{node} successfully', args, true),
  },
  'Restart Remote Nginx Error': {
    title: () => $gettext('Restart Remote Nginx Error'),
    content: (args: any) => $gettext('Restart Nginx on %{node} failed, response: %{resp}', args, true),
  },
  'Restart Remote Nginx Success': {
    title: () => $gettext('Restart Remote Nginx Success'),
    content: (args: any) => $gettext('Restart Nginx on %{node} successfully', args, true),
  },
  'Auto Backup Configuration Error': {
    title: () => $gettext('Auto Backup Configuration Error'),
    content: (args: any) => $gettext('Storage configuration validation failed for backup task %{backup_name}, error: %{error}', args, true),
  },
  'Auto Backup Failed': {
    title: () => $gettext('Auto Backup Failed'),
    content: (args: any) => $gettext('Backup task %{backup_name} failed to execute, error: %{error}', args, true),
  },
  'Auto Backup Storage Failed': {
    title: () => $gettext('Auto Backup Storage Failed'),
    content: (args: any) => $gettext('Backup task %{backup_name} failed during storage upload, error: %{error}', args, true),
  },
  'Auto Backup Completed': {
    title: () => $gettext('Auto Backup Completed'),
    content: (args: any) => $gettext('Backup task %{backup_name} completed successfully, file: %{file_path}', args, true),
  },
  'Certificate Expired': {
    title: () => $gettext('Certificate Expired'),
    content: (args: any) => $gettext('Certificate %{name} has expired', args, true),
  },
  'Certificate Expiration Notice': {
    title: () => $gettext('Certificate Expiration Notice'),
    content: (args: any) => $gettext('Certificate %{name} will expire in %{days} days', args, true),
  },
  'Certificate Expiring Soon': {
    title: () => $gettext('Certificate Expiring Soon'),
    content: (args: any) => $gettext('Certificate %{name} will expire in %{days} days', args, true),
  },
  'Certificate Expiring Soon_1': {
    title: () => $gettext('Certificate Expiring Soon'),
    content: (args: any) => $gettext('Certificate %{name} will expire in %{days} days', args, true),
  },
  'Certificate Expiring Soon_2': {
    title: () => $gettext('Certificate Expiring Soon'),
    content: (args: any) => $gettext('Certificate %{name} will expire in 1 day', args, true),
  },
  'Sync Certificate Error': {
    title: () => $gettext('Sync Certificate Error'),
    content: (args: any) => $gettext('Sync Certificate %{cert_name} to %{env_name} failed', args, true),
  },
  'Sync Certificate Success': {
    title: () => $gettext('Sync Certificate Success'),
    content: (args: any) => $gettext('Sync Certificate %{cert_name} to %{env_name} successfully', args, true),
  },
  'Sync Config Error': {
    title: () => $gettext('Sync Config Error'),
    content: (args: any) => $gettext('Sync config %{config_name} to %{env_name} failed', args, true),
  },
  'Sync Config Success': {
    title: () => $gettext('Sync Config Success'),
    content: (args: any) => $gettext('Sync config %{config_name} to %{env_name} successfully', args, true),
  },
  'Rename Remote Config Error': {
    title: () => $gettext('Rename Remote Config Error'),
    content: (args: any) => $gettext('Rename %{orig_path} to %{new_path} on %{env_name} failed', args, true),
  },
  'Rename Remote Config Success': {
    title: () => $gettext('Rename Remote Config Success'),
    content: (args: any) => $gettext('Rename %{orig_path} to %{new_path} on %{env_name} successfully', args, true),
  },
  'Delete Remote Config Error': {
    title: () => $gettext('Delete Remote Config Error'),
    content: (args: any) => $gettext('Delete %{path} on %{env_name} failed', args, true),
  },
  'Delete Remote Config Success': {
    title: () => $gettext('Delete Remote Config Success'),
    content: (args: any) => $gettext('Delete %{path} on %{env_name} successfully', args, true),
  },
  'Delete Remote Site Error': {
    title: () => $gettext('Delete Remote Site Error'),
    content: (args: any) => $gettext('Delete site %{name} from %{node} failed', args, true),
  },
  'Delete Remote Site Success': {
    title: () => $gettext('Delete Remote Site Success'),
    content: (args: any) => $gettext('Delete site %{name} from %{node} successfully', args, true),
  },
  'Disable Remote Site Error': {
    title: () => $gettext('Disable Remote Site Error'),
    content: (args: any) => $gettext('Disable site %{name} from %{node} failed', args, true),
  },
  'Disable Remote Site Success': {
    title: () => $gettext('Disable Remote Site Success'),
    content: (args: any) => $gettext('Disable site %{name} from %{node} successfully', args, true),
  },
  'Enable Remote Site Error': {
    title: () => $gettext('Enable Remote Site Error'),
    content: (args: any) => $gettext('Enable site %{name} on %{node} failed', args, true),
  },
  'Enable Remote Site Success': {
    title: () => $gettext('Enable Remote Site Success'),
    content: (args: any) => $gettext('Enable site %{name} on %{node} successfully', args, true),
  },
  'Enable Remote Site Maintenance Error': {
    title: () => $gettext('Enable Remote Site Maintenance Error'),
    content: (args: any) => $gettext('Enable site %{name} maintenance on %{node} failed', args, true),
  },
  'Enable Remote Site Maintenance Success': {
    title: () => $gettext('Enable Remote Site Maintenance Success'),
    content: (args: any) => $gettext('Enable site %{name} maintenance on %{node} successfully', args, true),
  },
  'Disable Remote Site Maintenance Error': {
    title: () => $gettext('Disable Remote Site Maintenance Error'),
    content: (args: any) => $gettext('Disable site %{name} maintenance on %{node} failed', args, true),
  },
  'Disable Remote Site Maintenance Success': {
    title: () => $gettext('Disable Remote Site Maintenance Success'),
    content: (args: any) => $gettext('Disable site %{name} maintenance on %{node} successfully', args, true),
  },
  'Rename Remote Site Error': {
    title: () => $gettext('Rename Remote Site Error'),
    content: (args: any) => $gettext('Rename site %{name} to %{new_name} on %{node} failed', args, true),
  },
  'Rename Remote Site Success': {
    title: () => $gettext('Rename Remote Site Success'),
    content: (args: any) => $gettext('Rename site %{name} to %{new_name} on %{node} successfully', args, true),
  },
  'Save Remote Site Error': {
    title: () => $gettext('Save Remote Site Error'),
    content: (args: any) => $gettext('Save site %{name} to %{node} failed', args, true),
  },
  'Save Remote Site Success': {
    title: () => $gettext('Save Remote Site Success'),
    content: (args: any) => $gettext('Save site %{name} to %{node} successfully', args, true),
  },
  'Delete Remote Stream Error': {
    title: () => $gettext('Delete Remote Stream Error'),
    content: (args: any) => $gettext('Delete stream %{name} from %{node} failed', args, true),
  },
  'Delete Remote Stream Success': {
    title: () => $gettext('Delete Remote Stream Success'),
    content: (args: any) => $gettext('Delete stream %{name} from %{node} successfully', args, true),
  },
  'Disable Remote Stream Error': {
    title: () => $gettext('Disable Remote Stream Error'),
    content: (args: any) => $gettext('Disable stream %{name} from %{node} failed', args, true),
  },
  'Disable Remote Stream Success': {
    title: () => $gettext('Disable Remote Stream Success'),
    content: (args: any) => $gettext('Disable stream %{name} from %{node} successfully', args, true),
  },
  'Enable Remote Stream Error': {
    title: () => $gettext('Enable Remote Stream Error'),
    content: (args: any) => $gettext('Enable stream %{name} on %{node} failed', args, true),
  },
  'Enable Remote Stream Success': {
    title: () => $gettext('Enable Remote Stream Success'),
    content: (args: any) => $gettext('Enable stream %{name} on %{node} successfully', args, true),
  },
  'Rename Remote Stream Error': {
    title: () => $gettext('Rename Remote Stream Error'),
    content: (args: any) => $gettext('Rename stream %{name} to %{new_name} on %{node} failed', args, true),
  },
  'Rename Remote Stream Success': {
    title: () => $gettext('Rename Remote Stream Success'),
    content: (args: any) => $gettext('Rename stream %{name} to %{new_name} on %{node} successfully', args, true),
  },
  'Save Remote Stream Error': {
    title: () => $gettext('Save Remote Stream Error'),
    content: (args: any) => $gettext('Save stream %{name} to %{node} failed', args, true),
  },
  'Save Remote Stream Success': {
    title: () => $gettext('Save Remote Stream Success'),
    content: (args: any) => $gettext('Save stream %{name} to %{node} successfully', args, true),
  },
  'All Recovery Codes Have Been Used': {
    title: () => $gettext('All Recovery Codes Have Been Used'),
    content: (args: any) => $gettext('Please generate new recovery codes in the preferences immediately to prevent lockout.', args, true),
  },
}

export default notifications
