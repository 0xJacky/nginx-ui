export interface HostAddress {
  host: string
  port: string
}

// Splits an SSH target such as "host:22", "[::1]:22" or "::1" into its host
// and port parts, defaulting the port to 22. Bare IPv6 literals without a
// bracket are treated as a host with no port.
export function parseHostAddress(address: string): HostAddress {
  const bracketed = address.match(/^\[([^\]]+)\](?::(\d+))?$/)
  if (bracketed) {
    return {
      host: bracketed[1],
      port: bracketed[2] ?? '22',
    }
  }

  const colonCount = (address.match(/:/g) ?? []).length
  if (colonCount === 1) {
    const [host, port] = address.split(':')
    return { host, port: port || '22' }
  }

  return { host: address, port: '22' }
}
