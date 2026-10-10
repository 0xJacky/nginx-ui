import type { RouteLocationRaw } from 'vue-router'
import type { SavableSettingsSection } from '@/api/settings'
import { SAVABLE_SETTINGS_SECTIONS } from '@/api/settings'

export type PreferenceGroupKey = 'basics' | 'features' | 'maintenance'

export type PreferenceSectionKey
  = | 'server'
    | 'app'
    | 'node'
    | 'http'
    | 'auth'
    | 'access_tokens'
    | 'cert'
    | 'nginx'
    | 'plugin'
    | 'openai'
    | 'health_check'
    | 'external_notify'
    | 'terminal'
    | 'logrotate'

export interface PreferenceSectionLink {
  label: string
  to: RouteLocationRaw
}

export interface PreferenceSection {
  key: PreferenceSectionKey
  group: PreferenceGroupKey
  label: string
  description: string
  link?: PreferenceSectionLink
  // Top-level keys of the settings object this section edits.
  // Used to place unsaved changes on the navigation.
  pathPrefixes: string[]
}

export interface PreferenceGroup {
  key: PreferenceGroupKey
  label: string
}

export const DEFAULT_SECTION_KEY: PreferenceSectionKey = 'server'

// Labels are resolved on every call so a language switch is reflected.
export function buildPreferenceGroups(): PreferenceGroup[] {
  return [
    { key: 'basics', label: $gettext('Basics') },
    { key: 'features', label: $gettext('Features') },
    { key: 'maintenance', label: $pgettext('preference group', 'Maintenance') },
  ]
}

export function buildPreferenceSections(): PreferenceSection[] {
  return [
    {
      key: 'server',
      group: 'basics',
      label: $gettext('Server'),
      description: $gettext('Address, protocol and certificate used to reach this Nginx UI instance.'),
      pathPrefixes: ['server', 'listener'],
    },
    {
      key: 'app',
      group: 'basics',
      label: $gettext('App'),
      description: $gettext('Sign-in session and list display of the web interface.'),
      pathPrefixes: ['app'],
    },
    {
      key: 'node',
      group: 'basics',
      label: $gettext('Node'),
      description: $gettext('Identity and display details of this node.'),
      link: { label: $gettext('Go to nodes'), to: '/nodes' },
      pathPrefixes: ['node'],
    },
    {
      key: 'http',
      group: 'basics',
      label: $gettext('HTTP'),
      description: $gettext('Proxies used when Nginx UI connects to the internet.'),
      pathPrefixes: ['http'],
    },
    {
      key: 'auth',
      group: 'basics',
      label: $gettext('Auth'),
      description: $gettext('Sign-in throttling, banned addresses and passkey origin.'),
      pathPrefixes: ['auth', 'webauthn', 'casdoor', 'oidc'],
    },
    {
      key: 'access_tokens',
      group: 'basics',
      label: $gettext('Access Tokens'),
      description: $gettext('Credentials for the command line, automation and MCP clients.'),
      pathPrefixes: [],
    },
    {
      key: 'cert',
      group: 'features',
      label: $gettext('Cert'),
      description: $gettext('Contact, issuing service and renewal of certificates requested by Nginx UI.'),
      link: { label: $gettext('Go to certificates'), to: '/certificates' },
      pathPrefixes: ['cert'],
    },
    {
      key: 'nginx',
      group: 'features',
      label: $gettext('Nginx'),
      description: $gettext('Paths, commands, maintenance page and control mode of the managed Nginx.'),
      pathPrefixes: ['nginx'],
    },
    {
      key: 'plugin',
      group: 'features',
      label: $gettext('Plugins'),
      description: $gettext('Plugin system, marketplace, packages and resource limits of plugin processes.'),
      link: { label: $gettext('Go to plugins'), to: '/system/plugins' },
      pathPrefixes: ['plugin'],
    },
    {
      key: 'openai',
      group: 'features',
      label: $gettext('LLM'),
      description: $gettext('Model and endpoint used by the assistant and code completion.'),
      pathPrefixes: ['openai'],
    },
    {
      key: 'health_check',
      group: 'features',
      label: $gettext('Health Check'),
      description: $gettext('Global switches and intervals for site and proxy target checks.'),
      link: { label: $gettext('Go to sites'), to: '/sites' },
      pathPrefixes: ['site_check', 'upstream_check'],
    },
    {
      key: 'external_notify',
      group: 'features',
      label: $gettext('External Notify'),
      description: $gettext('Channels that receive notifications from Nginx UI.'),
      link: { label: $gettext('Go to notifications'), to: '/notifications' },
      pathPrefixes: [],
    },
    {
      key: 'terminal',
      group: 'features',
      label: $gettext('Terminal'),
      description: $gettext('Shell started by the web terminal.'),
      link: { label: $gettext('Go to terminal'), to: '/terminal' },
      pathPrefixes: ['terminal'],
    },
    {
      key: 'logrotate',
      group: 'maintenance',
      label: $gettext('Logrotate'),
      description: $gettext('Scheduled rotation of Nginx log files.'),
      pathPrefixes: ['logrotate'],
    },
  ]
}

export function findSectionForPath(sections: PreferenceSection[], path: string) {
  const prefix = path.split('.')[0]
  return sections.find(section => section.pathPrefixes.includes(prefix))
}

// The preference section each savable settings section is edited on.
const settingsSectionTab: Record<SavableSettingsSection, PreferenceSectionKey> = {
  app: 'app',
  server: 'server',
  auth: 'auth',
  oidc: 'auth',
  cert: 'cert',
  http: 'http',
  node: 'node',
  openai: 'openai',
  logrotate: 'logrotate',
  nginx: 'nginx',
  plugin: 'plugin',
  site_check: 'health_check',
  upstream_check: 'health_check',
}

// The settings sections edited on a preference section.
export function tabSettingsSections(tab: string): SavableSettingsSection[] {
  return SAVABLE_SETTINGS_SECTIONS.filter(section => settingsSectionTab[section] === tab)
}
