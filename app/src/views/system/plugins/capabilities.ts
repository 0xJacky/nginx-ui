export interface CapabilityPreset {
  label: () => string
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
    label: () => $gettext('Certificate DNS validation'),
    description: () => $gettext('Validates domains through DNS records when certificates are issued or renewed.'),
    link: { to: '/dns/credentials', label: () => $gettext('DNS credentials') },
  },
  'http': {
    label: () => $gettext('Web pages and API'),
    description: () => $gettext('Serves its own pages and API inside Nginx UI.'),
  },
  'notify': {
    label: () => $gettext('External notifications'),
    description: () => $gettext('Delivers Nginx UI notifications to other services.'),
  },
  'probe': {
    label: () => $gettext('Health checks'),
    description: () => $gettext('Adds more ways to check whether sites and services are reachable.'),
  },
  'mcp': {
    label: () => $gettext('AI assistant tools'),
    description: () => $gettext('Offers actions that AI assistants connected to Nginx UI can run.'),
  },
  'storage': {
    label: () => $gettext('Backup storage'),
    description: () => $gettext('Stores backups and other files in external storage.'),
    link: { to: '/backup/auto-backup', label: () => $gettext('Auto backup') },
  },
  'cert.deploy': {
    label: () => $gettext('Certificate deployment'),
    description: () => $gettext('Pushes issued certificates to other services and devices.'),
    link: { to: '/certificates/deploy_targets', label: () => $gettext('Deploy targets') },
  },
  'security.blocklist': {
    label: () => $gettext('Blocklists'),
    description: () => $gettext('Keeps lists of addresses that are denied access up to date.'),
    link: { to: '/blocklists', label: () => $gettext('Blocklists') },
  },
  'upstream.discovery': {
    label: () => $gettext('Service discovery'),
    description: () => $gettext('Finds backend servers automatically and keeps upstreams up to date.'),
  },
  'log.sink': {
    label: () => $gettext('Access log forwarding'),
    description: () => $gettext('Receives access log entries as they are written, for example to forward or analyse them.'),
  },
}

/** Preset of a capability, a generic one for a capability the host does not know. */
export function capabilityPreset(capability: string): CapabilityPreset {
  return capabilityPresets[capability] ?? {
    label: () => $gettext('Other feature'),
    description: () => $gettext('A feature this version of Nginx UI does not describe.'),
  }
}

export function capabilityLabel(capability: string): string {
  return capabilityPreset(capability).label()
}
