export function isIPv4Address(value: string): boolean {
  const parts = value.split('.')
  return parts.length === 4 && parts.every(part => {
    if (!/^\d{1,3}$/.test(part))
      return false

    const number = Number(part)
    return number >= 0 && number <= 255
  })
}

export function isIPv6Address(value: string): boolean {
  const candidate = value.startsWith('[') && value.endsWith(']')
    ? value.slice(1, -1)
    : value
  if (!candidate.includes(':'))
    return false

  try {
    const url = new URL(`http://[${candidate}]/`)
    return url.hostname.startsWith('[') && url.hostname.endsWith(']')
  }
  catch {
    return false
  }
}

export function isIPAddress(value: string): boolean {
  const candidate = value.trim()
  return isIPv4Address(candidate) || isIPv6Address(candidate)
}

export function splitCertificateIdentifiers(values: string[]): string[] {
  return [...new Set(values.flatMap(value => value.split(/\s+/))
    .map(value => value.trim())
    .filter(value => value && value !== '_'))]
}
