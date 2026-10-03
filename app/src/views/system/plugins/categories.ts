/**
 * Plain wording of the catalog categories, in the order the marketplace
 * offers them. A category this host does not know shows its id.
 */
const categoryLabels: Record<string, () => string> = {
  certificates: () => $gettext('Certificates'),
  dns: () => $gettext('DNS'),
  security: () => $gettext('Security'),
  traffic: () => $gettext('Traffic and upstreams'),
  monitoring: () => $gettext('Monitoring'),
  logs: () => $gettext('Logs'),
  analytics: () => $gettext('Analytics'),
  notifications: () => $gettext('Notifications'),
  backup: () => $gettext('Backup and storage'),
  ai: () => $gettext('AI'),
  templates: () => $gettext('Config templates'),
  languages: () => $gettext('Language packs'),
  integrations: () => $gettext('Integrations'),
  tools: () => $gettext('Tools'),
}

export function categoryLabel(id: string): string {
  return categoryLabels[id]?.() ?? id
}

/** Known categories in their order, any other after them by id. */
export function sortCategories(ids: Iterable<string>): string[] {
  const used = new Set(ids)
  const known = Object.keys(categoryLabels).filter(id => used.has(id))
  return [...known, ...[...used].filter(id => !(id in categoryLabels)).sort()]
}
