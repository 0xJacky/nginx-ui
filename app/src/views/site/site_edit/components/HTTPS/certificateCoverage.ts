import { toUnicodeDomain } from '@/utils/idnDomain'

// Pure helpers for the "Existing certificate" method of the HTTPS card. They
// only preview what the backend decides (it reads the SANs of the certificate
// file); the UI uses them to warn before a run.

export function normalizeCertificateName(value: string): string {
  // Compare IDNs in one form: the record may hold punycode, the site Unicode.
  return toUnicodeDomain(value.trim().replace(/\.$/, '')).toLowerCase()
}

/**
 * Reports whether one certificate name covers one requested identifier. A
 * wildcard `*.example.com` covers exactly one extra label (`a.example.com`),
 * neither `example.com` nor `a.b.example.com`. A requested wildcard is only
 * covered by the same wildcard.
 */
export function certificateNameCovers(certificateName: string, identifier: string): boolean {
  const name = normalizeCertificateName(certificateName)
  const target = normalizeCertificateName(identifier)
  if (!name || !target)
    return false
  if (name === target)
    return true
  if (!name.startsWith('*.') || target.startsWith('*.'))
    return false

  const suffix = name.slice(1) // `.example.com`
  if (!target.endsWith(suffix))
    return false

  const label = target.slice(0, -suffix.length)
  return label !== '' && !label.includes('.')
}

export interface CertificateCoverage {
  // False when the record lists no names, so nothing can be told client-side.
  known: boolean
  covered: string[]
  uncovered: string[]
}

export function certificateCoverage(certificateNames: readonly string[] | undefined | null, identifiers: readonly string[]): CertificateCoverage {
  const names = (certificateNames ?? []).filter(name => name?.trim())
  if (!names.length)
    return { known: false, covered: [], uncovered: [] }

  const covered: string[] = []
  const uncovered: string[] = []
  identifiers.forEach(identifier => {
    if (names.some(name => certificateNameCovers(name, identifier)))
      covered.push(identifier)
    else
      uncovered.push(identifier)
  })

  return { known: true, covered, uncovered }
}

/** True when `notAfter` is a known date in the past. An unknown expiry is not expired. */
export function isCertificateExpired(notAfter: string | undefined | null, now: Date = new Date()): boolean {
  if (!notAfter)
    return false

  const time = Date.parse(notAfter)
  // Go's zero time.Time (0001-01-01) means the backend could not read the file.
  if (Number.isNaN(time) || new Date(time).getUTCFullYear() <= 1)
    return false

  return time <= now.getTime()
}
