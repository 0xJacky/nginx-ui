import type { PluginManifestI18n } from '@/api/plugin'
import { localizedText } from '@/api/plugin'

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
      return $gettext('Keep its own data inside Nginx UI.')
    case 'network':
      return $gettext('Connect to other servers over the network.')
    case 'cron':
      return $gettext('Run tasks on a regular schedule.')
    case 'notify':
      return $gettext('Send notifications through the Nginx UI notification channels.')
    case 'metrics.read':
      return $gettext('Read CPU, memory, network and Nginx status metrics of this node.')
    case 'core_api':
      return $gettext('Manage Nginx UI with your permissions.')
    case 'mcp':
      return $gettext('Offer actions that AI assistants connected to Nginx UI can run.')
    case 'cert.deploy':
      return $gettext('Receive issued certificates, including the parts that must stay secret, to push them to other services.')
    case 'log.read':
      return $gettext('Receive every access log entry while it is on, including visitor addresses and requested URLs.')
    case 'log.files':
      return $gettext('Read the Nginx log files of this node, including visitor addresses and requested URLs.')
    case 'nginx.snippet':
      return $gettext('Add Nginx configuration of its own. It takes effect only where you include it, and Nginx UI applies it only when the configuration remains valid.')
    case 'nginx.config.read':
      return $gettext('Read the Nginx configuration files of this node.')
    case 'sites.read':
      return $gettext('See the sites of this node and their addresses.')
    case 'certs.read':
      return $gettext('See the certificates and when they expire, without the parts that must stay secret.')
    default:
      return $gettext('Unknown permission. Only grant it if you trust the plugin author.')
  }
}

/** Short name of a permission, never the raw permission id. */
export function permissionLabel(permission: string): string {
  if (permission.startsWith('credentials.read:')) {
    return $gettext('Stored %{kind} credentials', {
      kind: permission.slice('credentials.read:'.length) || $gettext('unknown'),
    })
  }

  switch (permission) {
    case 'kv':
      return $gettext('Own data storage')
    case 'network':
      return $gettext('Network')
    case 'cron':
      return $gettext('Scheduled tasks')
    case 'notify':
      return $gettext('Notifications')
    case 'metrics.read':
      return $gettext('Server metrics')
    case 'core_api':
      return $gettext('Nginx UI management')
    case 'mcp':
      return $gettext('AI assistant tools')
    case 'cert.deploy':
      return $gettext('Certificates')
    case 'log.read':
      return $gettext('Access logs')
    case 'log.files':
      return $gettext('Log files')
    case 'nginx.snippet':
      return $gettext('Nginx configuration snippets')
    case 'nginx.config.read':
      return $gettext('Nginx configuration')
    case 'sites.read':
      return $gettext('Sites')
    case 'certs.read':
      return $gettext('Certificate list')
    default:
      return $gettext('Unknown permission')
  }
}

/** A manifest or an installed plugin, whichever carries the reasons. */
export interface PermissionReasonSource {
  permission_reasons?: Record<string, string>
  permission_reasons_i18n?: Record<string, Record<string, string>>
  i18n?: Record<string, PluginManifestI18n>
}

/**
 * The reason a plugin gives for each permission, in the given language. An
 * empty reason counts as none, and an empty translation falls back.
 */
export function permissionReasons(source: PermissionReasonSource | undefined, language: string): Record<string, string> {
  const byLocale: Record<string, Record<string, string>> = { ...source?.permission_reasons_i18n }
  for (const [locale, translated] of Object.entries(source?.i18n ?? {})) {
    if (translated?.permission_reasons)
      byLocale[locale] = { ...byLocale[locale], ...translated.permission_reasons }
  }

  const names = new Set(Object.keys(source?.permission_reasons ?? {}))
  for (const translated of Object.values(byLocale))
    Object.keys(translated).forEach(name => names.add(name))

  const reasons: Record<string, string> = {}
  for (const name of names) {
    const translations: Record<string, string> = {}
    for (const [locale, translated] of Object.entries(byLocale)) {
      if (translated[name]?.trim())
        translations[locale] = translated[name]
    }
    const text = localizedText(translations, language, source?.permission_reasons?.[name] ?? '').trim()
    if (text)
      reasons[name] = text
  }
  return reasons
}
