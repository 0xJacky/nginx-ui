export type UsageKind = 'site' | 'stream' | 'config'

export interface Usage {
  /** Path relative to the Nginx configuration directory. */
  path: string
  label: string
  kind: UsageKind
  /** The page that edits the file. */
  to: string
}

/** Turns the files that include a snippet into links to their editors. */
export function toUsages(usedBy: string[]): Usage[] {
  return usedBy.map(path => {
    const [dir, ...rest] = path.split('/')
    const name = rest.join('/')
    if (dir === 'sites-available')
      return { path, label: name, kind: 'site', to: `/sites/${encodeURIComponent(name)}` }
    if (dir === 'streams-available')
      return { path, label: name, kind: 'stream', to: `/streams/${encodeURIComponent(name)}` }
    return { path, label: path, kind: 'config', to: `/config/${path}/edit` }
  })
}

export const usageColor: Record<UsageKind, string> = { site: 'blue', stream: 'purple', config: 'default' }
