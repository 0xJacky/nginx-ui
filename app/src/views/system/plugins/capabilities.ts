export interface CapabilityPreset {
  label: () => string
  /** Tabler icon class shown next to the label on the plugin cards. */
  icon: string
  description: () => string
  /** Host page where the feature is used, if there is one. */
  link?: {
    to: string
    label: () => string
  }
}

/**
 * Plain wording of every capability the host knows, so the page never shows
 * the raw capability ids.
 */
const capabilityPresets: Record<string, CapabilityPreset> = {
  'dns01': {
    icon: 'i-tabler-world-www',
    label: () => $gettext('Certificate DNS validation'),
    description: () => $gettext('Validates domains through DNS records when certificates are issued or renewed.'),
    link: { to: '/dns/credentials', label: () => $gettext('DNS credentials') },
  },
  'http': {
    icon: 'i-tabler-browser',
    label: () => $gettext('Web pages and API'),
    description: () => $gettext('Serves its own pages and API inside Nginx UI.'),
  },
  'notify': {
    icon: 'i-tabler-bell',
    label: () => $gettext('External notifications'),
    description: () => $gettext('Delivers Nginx UI notifications to other services.'),
  },
  'probe': {
    icon: 'i-tabler-heartbeat',
    label: () => $gettext('Health checks'),
    description: () => $gettext('Adds more ways to check whether sites and services are reachable.'),
  },
  'mcp': {
    icon: 'i-tabler-robot',
    label: () => $gettext('AI assistant tools'),
    description: () => $gettext('Offers actions that AI assistants connected to Nginx UI can run.'),
  },
  'storage': {
    icon: 'i-tabler-database',
    label: () => $gettext('Backup storage'),
    description: () => $gettext('Stores backups and other files in external storage.'),
    link: { to: '/backup/auto-backup', label: () => $gettext('Auto backup') },
  },
  'cert.deploy': {
    icon: 'i-tabler-certificate',
    label: () => $gettext('Certificate deployment'),
    description: () => $gettext('Pushes issued certificates to other services and devices.'),
    link: { to: '/certificates/deploy_targets', label: () => $gettext('Deploy targets') },
  },
  'security.blocklist': {
    icon: 'i-tabler-ban',
    label: () => $gettext('Blocklists'),
    description: () => $gettext('Keeps lists of addresses that are denied access up to date.'),
    link: { to: '/blocklists', label: () => $gettext('Blocklists') },
  },
  'upstream.discovery': {
    icon: 'i-tabler-radar',
    label: () => $gettext('Service discovery'),
    description: () => $gettext('Finds backend servers automatically and keeps upstreams up to date.'),
  },
  'log.sink': {
    icon: 'i-tabler-file-analytics',
    label: () => $gettext('Access log forwarding'),
    description: () => $gettext('Receives access log entries as they are written, for example to forward or analyse them.'),
  },
}

/** Preset of a capability, a generic one for a capability the host does not know. */
export function capabilityPreset(capability: string): CapabilityPreset {
  return capabilityPresets[capability] ?? {
    icon: 'i-tabler-plug',
    label: () => $gettext('Other feature'),
    description: () => $gettext('A feature this version of Nginx UI does not describe.'),
  }
}

export function capabilityLabel(capability: string): string {
  return capabilityPreset(capability).label()
}

export function capabilityIcon(capability: string): string {
  return capabilityPreset(capability).icon
}
