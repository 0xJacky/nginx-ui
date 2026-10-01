import type { SyncSummary } from '@/api/cluster_sync'
import type { TemplateOrigin, Variable } from '@/api/template'
import { http } from '@uozi-admin/request'

export interface Snippet {
  /** File name in the snippets directory, such as "gzip.conf". */
  file: string
  /** The name to show without a language at hand. */
  name: string
  /** The name per language. */
  name_i18n: Record<string, string>
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
  name_i18n: Record<string, string>
  description: Record<string, string>
  /** Left out, the author of the snippet is kept. */
  author?: string
  /** Left out, the variables of the snippet are kept. */
  variables?: Record<string, Variable>
  content: string
}

export interface SnippetPreview {
  content: string
  /** Why the content does not render, or does not parse as Nginx configuration. */
  error?: string
}

/** A block template built into Nginx UI or offered by an enabled plugin, as written. */
export interface BuiltinTemplate {
  name: string
  description: Record<string, string>
  author: string
  /** File name of the template, such as "hsts.conf". */
  filename: string
  variables: Record<string, Variable>
  origin?: TemplateOrigin
  /** The plugin a template of origin "plugin" comes from. */
  plugin_id?: string
  /** The body below the header, only when one template is read. */
  content?: string
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
  preview: (content: string, variables: Record<string, Variable>, fromPlugin = false) => http.post<SnippetPreview>('/snippet_preview', { content, variables, from_plugin: fromPlugin }),
  getBuiltins: () => http.get<{ data: BuiltinTemplate[] }>('/snippet_templates'),
  getBuiltin: (name: string, pluginId?: string) => http.get<BuiltinTemplate>(`/snippet_templates/${encodeURIComponent(name)}`, { params: pluginId ? { plugin_id: pluginId } : undefined }),
  delete: (file: string) => http.delete(`/snippets/${encodeURIComponent(file)}`),
  getSync: () => http.get<SnippetSync>('/snippet_sync'),
  saveSync: (payload: SnippetSync) => http.post<SyncSummary>('/snippet_sync', payload),
}

export default snippet
