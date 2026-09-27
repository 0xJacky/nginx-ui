import { challtestsrvURL } from './env'

// pebble-challtestsrv answers Pebble's DNS queries. Every name resolves to the
// nginx-ui container unless a test overrides it through the management API.

function fqdn(domain: string) {
  return domain.endsWith('.') ? domain : `${domain}.`
}

async function manage(path: string, body: unknown) {
  const response = await fetch(`${challtestsrvURL}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!response.ok)
    throw new Error(`challtestsrv ${path} answered ${response.status}: ${await response.text()}`)
}

/** Resolve <domain> to the given IPv4 addresses for Pebble's validation. */
export async function pointDomainTo(domain: string, ...addresses: string[]) {
  await manage('/add-a', { host: fqdn(domain), addresses })
}

/** Drop an override so <domain> resolves to the nginx-ui container again. */
export async function clearDomain(domain: string) {
  await manage('/clear-a', { host: fqdn(domain) })
}
