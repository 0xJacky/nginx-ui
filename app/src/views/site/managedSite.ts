// The name of the ingress a site is generated from (`managed_by` is
// "ingress:<name>"), or '' for a regular site. Kept out of @/api/site so tests
// that mock the API module still get it.
export function managedIngressName(managedBy?: string | null): string {
  if (!managedBy)
    return ''
  return managedBy.startsWith('ingress:') ? managedBy.slice('ingress:'.length) : managedBy
}
