import type { SyncSummary } from '@/api/cluster_sync'
import type { Variable } from '@/api/template'
import { http } from '@uozi-admin/request'

export interface Snippet {
  /** File name in the snippets directory, such as "gzip.conf". */
  file: string
  name: string
  description: Record<string, string>
  author: string
  variables: Record<string, Variable>
  /** The directive that includes the snippet. */
  include: string
  modified_at: string
  /** Configuration files that include the snippet, relative to the Nginx configuration directory. */
  used_by: string[]
  /** The body without the header, only when one snippet is read. */
  content?: string
}

export interface SnippetPayload {
  file?: string
  name: string
  description: Record<string, string>
  content: string
}

export interface SnippetSync {
  sync_node_ids: number[]
  sync_overwrite: boolean
}

const snippet = {
  getList: () => http.get<{ data: Snippet[] }>('/snippets'),
  get: (file: string) => http.get<Snippet>(`/snippets/${encodeURIComponent(file)}`),
  create: (payload: SnippetPayload) => http.post<Snippet>('/snippets', payload),
  update: (file: string, payload: SnippetPayload) => http.post<Snippet>(`/snippets/${encodeURIComponent(file)}`, payload),
  delete: (file: string) => http.delete(`/snippets/${encodeURIComponent(file)}`),
  getSync: () => http.get<SnippetSync>('/snippet_sync'),
  saveSync: (payload: SnippetSync) => http.post<SyncSummary>('/snippet_sync', payload),
}

export default snippet
