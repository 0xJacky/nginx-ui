/**
 * Plain English explanation of every host permission a plugin can request, so
 * the install and approval dialogs say what granting it actually allows.
 */
export function describePermission(permission: string): string {
  if (permission.startsWith('credentials.read:')) {
    return $gettext('Read stored %{kind} credentials, including their secrets.', {
      kind: permission.slice('credentials.read:'.length) || $gettext('unknown'),
    })
  }

  switch (permission) {
    case 'kv':
      return $gettext('Store and read its own key-value data inside the Nginx UI database.')
    case 'network':
      return $gettext('Make outbound network requests to the hosts declared in its manifest.')
    case 'cron':
      return $gettext('Run scheduled tasks on the schedules declared in its manifest.')
    case 'notify':
      return $gettext('Send notifications through the Nginx UI notification channels.')
    case 'metrics.read':
      return $gettext('Read CPU, memory, network and Nginx status metrics of this node.')
    case 'core_api':
      return $gettext('Call the Nginx UI management API with your permissions.')
    case 'mcp':
      return $gettext('Offer its tools to AI assistants connected to Nginx UI through MCP, which can then run them.')
    default:
      return $gettext('Unknown permission. Only grant it if you trust the plugin author.')
  }
}
