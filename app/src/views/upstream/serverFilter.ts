export interface FilterableServer {
  address: string
  // host:port key of the availability results, when the backend knows it
  socket?: string
  down?: boolean
  backup?: boolean
}

export interface FilterableUpstream<S extends FilterableServer = FilterableServer> {
  name: string
  servers: S[]
}

export function normalizeKeyword(keyword: string | null | undefined) {
  return (keyword ?? '').trim().toLowerCase()
}

function serverMatches(server: FilterableServer, needle: string) {
  return server.address.toLowerCase().includes(needle)
    || (server.socket?.toLowerCase().includes(needle) ?? false)
}

/**
 * Narrows upstream groups to a search keyword. A group whose name matches is
 * kept with all of its servers; otherwise only the servers whose address (or
 * host:port socket) contains the keyword are kept, and groups without any are
 * dropped. The input is never mutated.
 */
export function filterUpstreams<T extends FilterableUpstream>(groups: T[], keyword: string | null | undefined): T[] {
  const needle = normalizeKeyword(keyword)
  if (!needle)
    return groups

  const result: T[] = []
  for (const group of groups) {
    if (group.name.toLowerCase().includes(needle)) {
      result.push(group)
      continue
    }
    const servers = group.servers.filter(server => serverMatches(server, needle))
    if (servers.length > 0)
      result.push({ ...group, servers })
  }
  return result
}

/**
 * Reports whether turning off the server at address leaves the group without
 * any enabled primary server, so every request would fail with a 502 unless a
 * backup server takes over.
 */
export function disablesLastPrimary(servers: FilterableServer[], address: string) {
  const target = servers.find(server => server.address === address)
  if (!target || target.down || target.backup)
    return false
  return !servers.some(server => server.address !== address && !server.down && !server.backup)
}
