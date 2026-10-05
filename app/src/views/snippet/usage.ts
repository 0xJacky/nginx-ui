import type { ConfigUsageItem } from '@/components/ConfigUsage'

/** Turns the files that include a snippet into links to their editors. */
export function toUsages(usedBy: string[]): ConfigUsageItem[] {
  return usedBy.map(path => {
    const [dir, ...rest] = path.split('/')
    const name = rest.join('/')
    if (dir === 'sites-available')
      return { key: path, label: name, kind: 'site', to: `/sites/${encodeURIComponent(name)}` }
    if (dir === 'streams-available')
      return { key: path, label: name, kind: 'stream', to: `/streams/${encodeURIComponent(name)}` }
    return { key: path, label: path, kind: 'config', to: `/config/${path}/edit` }
  })
}
