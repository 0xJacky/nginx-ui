import type { DNSProvider } from '@/api/auto_cert'

export const ALLOWED_DNS_PROVIDER_CODES = ['alidns', 'tencentcloud', 'cloudflare'] as const

type DNSProviderIdentifier = Pick<DNSProvider, 'code' | 'provider' | 'name'> | null

const allowedProviderSet = new Set<string>(ALLOWED_DNS_PROVIDER_CODES)

export function normalizeProviderCode(value?: string | null) {
  return (value ?? '').trim().toLowerCase()
}

export function resolveProviderCode(provider?: DNSProviderIdentifier) {
  if (!provider)
    return ''
  return normalizeProviderCode(provider.code ?? provider.provider ?? provider.name)
}

export function isAllowedDnsProvider(provider?: DNSProviderIdentifier) {
  const code = resolveProviderCode(provider)
  return Boolean(code) && allowedProviderSet.has(code)
}

export function isAllowedDnsProviderCode(value?: string | null) {
  const normalized = normalizeProviderCode(value)
  return Boolean(normalized) && allowedProviderSet.has(normalized)
}

export function filterAllowedDnsProviders<T extends Pick<DNSProvider, 'code' | 'provider' | 'name'>>(list: T[]) {
  return list.filter(item => isAllowedDnsProvider(item))
}
