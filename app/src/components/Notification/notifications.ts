// Auto-generated notification texts
// Extracted from Go source code notification function calls
/* eslint-disable ts/no-explicit-any */

const notifications: Record<string, { title: () => string, content: (args: any) => string }> = {

  // cert module notifications
  'Sync Certificate Error': {
    title: () => $gettext('Sync Certificate Error'),
    content: (args: any) => $gettext('Sync Certificate %{cert_name} to %{env_name} failed', args),
  },
  'Sync Certificate Success': {
    title: () => $gettext('Sync Certificate Success'),
    content: (args: any) => $gettext('Sync Certificate %{cert_name} to %{env_name} successfully', args),
  },

  // config module notifications
  'Sync Config Error': {
    title: () => $gettext('Sync Config Error'),
    content: (args: any) => $gettext('Sync config %{config_name} to %{env_name} failed', args),
  },
  'Sync Config Success': {
    title: () => $gettext('Sync Config Success'),
    content: (args: any) => $gettext('Sync config %{config_name} to %{env_name} successfully', args),
  },
  'Rename Remote Config Error': {
    title: () => $gettext('Rename Remote Config Error'),
    content: (args: any) => $gettext('Rename %{orig_path} to %{new_path} on %{env_name} failed', args),
  },
  'Rename Remote Config Success': {
    title: () => $gettext('Rename Remote Config Success'),
    content: (args: any) => $gettext('Rename %{orig_path} to %{new_path} on %{env_name} successfully', args),
  },

  // site module notifications
  'Delete Remote Site Error': {
    title: () => $gettext('Delete Remote Site Error'),
    content: (args: any) => $gettext('Delete site %{name} from %{node} failed', args),
  },
  'Delete Remote Site Success': {
    title: () => $gettext('Delete Remote Site Success'),
    content: (args: any) => $gettext('Delete site %{name} from %{node} successfully', args),
  },
  'Disable Remote Site Error': {
    title: () => $gettext('Disable Remote Site Error'),
    content: (args: any) => $gettext('Disable site %{name} from %{node} failed', args),
  },
  'Disable Remote Site Success': {
    title: () => $gettext('Disable Remote Site Success'),
    content: (args: any) => $gettext('Disable site %{name} from %{node} successfully', args),
  },
  'Enable Remote Site Error': {
    title: () => $gettext('Enable Remote Site Error'),
    content: (args: any) => $gettext('Enable site %{name} on %{node} failed', args),
  },
  'Enable Remote Site Success': {
    title: () => $gettext('Enable Remote Site Success'),
    content: (args: any) => $gettext('Enable site %{name} on %{node} successfully', args),
  },
  'Rename Remote Site Error': {
    title: () => $gettext('Rename Remote Site Error'),
    content: (args: any) => $gettext('Rename site %{name} to %{new_name} on %{node} failed', args),
  },
  'Rename Remote Site Success': {
    title: () => $gettext('Rename Remote Site Success'),
    content: (args: any) => $gettext('Rename site %{name} to %{new_name} on %{node} successfully', args),
  },
  'Save Remote Site Error': {
    title: () => $gettext('Save Remote Site Error'),
    content: (args: any) => $gettext('Save site %{name} to %{node} failed', args),
  },
  'Save Remote Site Success': {
    title: () => $gettext('Save Remote Site Success'),
    content: (args: any) => $gettext('Save site %{name} to %{node} successfully', args),
  },

  // stream module notifications
  'Delete Remote Stream Error': {
    title: () => $gettext('Delete Remote Stream Error'),
    content: (args: any) => $gettext('Delete stream %{name} from %{node} failed', args),
  },
  'Delete Remote Stream Success': {
    title: () => $gettext('Delete Remote Stream Success'),
    content: (args: any) => $gettext('Delete stream %{name} from %{node} successfully', args),
  },
  'Disable Remote Stream Error': {
    title: () => $gettext('Disable Remote Stream Error'),
    content: (args: any) => $gettext('Disable stream %{name} from %{node} failed', args),
  },
  'Disable Remote Stream Success': {
    title: () => $gettext('Disable Remote Stream Success'),
    content: (args: any) => $gettext('Disable stream %{name} from %{node} successfully', args),
  },
  'Enable Remote Stream Error': {
    title: () => $gettext('Enable Remote Stream Error'),
    content: (args: any) => $gettext('Enable stream %{name} on %{node} failed', args),
  },
  'Enable Remote Stream Success': {
    title: () => $gettext('Enable Remote Stream Success'),
    content: (args: any) => $gettext('Enable stream %{name} on %{node} successfully', args),
  },
  'Rename Remote Stream Error': {
    title: () => $gettext('Rename Remote Stream Error'),
    content: (args: any) => $gettext('Rename stream %{name} to %{new_name} on %{node} failed', args),
  },
  'Rename Remote Stream Success': {
    title: () => $gettext('Rename Remote Stream Success'),
    content: (args: any) => $gettext('Rename stream %{name} to %{new_name} on %{node} successfully', args),
  },
  'Save Remote Stream Error': {
    title: () => $gettext('Save Remote Stream Error'),
    content: (args: any) => $gettext('Save stream %{name} to %{node} failed', args),
  },
  'Save Remote Stream Success': {
    title: () => $gettext('Save Remote Stream Success'),
    content: (args: any) => $gettext('Save stream %{name} to %{node} successfully', args),
  },

  // user module notifications
  'All Recovery Codes Have Been Used': {
    title: () => $gettext('All Recovery Codes Have Been Used'),
    content: (args: any) => $gettext('Please generate new recovery codes in the preferences immediately to prevent lockout.', args),
  },
}

export default notifications
