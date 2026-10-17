import type { ModelBase } from '@/api/curd'
import type { ConfigurationField } from '@/api/plugin'
import { http, useCurdApi } from '@uozi-admin/request'

/**
 * Interface for restore backup response
 */
export interface RestoreResponse {
  restore_dir: string
  nginx_ui_restored: boolean
  nginx_restored: boolean
  hash_match: boolean
}

/**
 * Interface for restore backup options
 */
export interface RestoreOptions {
  backup_file: File
  security_token: string
  restore_nginx: boolean
  restore_nginx_ui: boolean
  verify_hash: boolean
}

export interface RestoreAccessOptions {
  installSecret?: string
  setupAuth?: boolean
}

function installSecretHeaders(installSecret?: string) {
  if (!installSecret) {
    return {}
  }

  return {
    'X-Install-Secret': installSecret,
  }
}

function getRestorePath(options?: RestoreAccessOptions) {
  return options?.setupAuth ? '/setup/restore' : '/restore'
}

/**
 * Where an auto backup stores its runs: the built-in local or S3 storage, or
 * a backend a storage plugin provides, stored as "plugin:<code>".
 */
export type StorageType = 'local' | 's3' | `plugin:${string}`

/**
 * Interface for auto backup configuration
 */
export interface AutoBackup extends ModelBase {
  name: string
  backup_type: 'nginx_config' | 'nginx_ui_config' | 'both_config' | 'custom_dir' | 'nginx_and_nginx_ui'
  storage_type: StorageType
  backup_path?: string
  storage_path: string
  cron_expression: string
  enabled: boolean
  retention_count: number
  last_backup_time?: string
  last_backup_status: 'pending' | 'success' | 'failed'
  last_backup_error?: string
  s3_endpoint?: string
  s3_access_key_id?: string
  s3_secret_access_key?: string
  s3_bucket?: string
  s3_region?: string
  /** Form values of a plugin storage backend. */
  storage_config?: Record<string, string>
}

/** A storage type an auto backup can use, with the form of a plugin backend. */
export interface StorageBackend {
  type: string
  name: string
  builtin?: boolean
  plugin_id?: string
  fields: ConfigurationField[]
}

/** One run a plugin storage backend keeps for a task. */
export interface StoredBackup {
  key: string
  key_file?: string
  size: number
  created_at: string
  modified_at?: string
}

export interface RestoreStoredOptions {
  key: string
  restore_nginx: boolean
  restore_nginx_ui: boolean
  verify_hash: boolean
}

const backup = {
  /**
   * Create and download a backup of nginx-ui and nginx configurations
   * Use http module with returnFullResponse option to access headers
   */
  createBackup() {
    return http.get('/backup', {
      responseType: 'blob',
      returnFullResponse: true,
    })
  },

  /**
   * Restore from a backup file
   * @param options RestoreOptions
   */
  restoreBackup(options: RestoreOptions, accessOptions?: RestoreAccessOptions) {
    const formData = new FormData()
    formData.append('backup_file', options.backup_file)
    formData.append('security_token', options.security_token)
    formData.append('restore_nginx', options.restore_nginx.toString())
    formData.append('restore_nginx_ui', options.restore_nginx_ui.toString())
    formData.append('verify_hash', options.verify_hash.toString())

    return http.post(getRestorePath(accessOptions), formData, {
      headers: {
        'Content-Type': 'multipart/form-data;charset=UTF-8',
        ...installSecretHeaders(accessOptions?.installSecret),
      },
      crypto: true,
      skipAuthRedirect: !!accessOptions?.setupAuth,
    })
  },
}

/**
 * Test S3 connection for auto backup configuration
 * @param config AutoBackup configuration with S3 settings
 */
export function testS3Connection(config: AutoBackup) {
  return http.post('/auto_backup/test_s3', config)
}

/**
 * Immediately execute a saved auto backup configuration.
 * @param id Auto backup configuration ID
 */
export function runAutoBackup(id: number) {
  return http.post(`/auto_backup/${id}/run`, undefined, {
    skipErrHandling: true,
  })
}

/** Lists the built-in storage and the backends storage plugins provide. */
export function listStorageBackends(): Promise<{ data: StorageBackend[] }> {
  return http.get('/auto_backup/storage_backends')
}

/** Validates a plugin storage backend configuration and lists its key prefix. */
export function testPluginStorage(config: AutoBackup): Promise<{ message: string, stored: number }> {
  return http.post('/auto_backup/test_storage', config)
}

/** Lists the runs the plugin storage backend of a task keeps, newest first. */
export function listStoredBackups(id: number): Promise<{ data: StoredBackup[] }> {
  return http.get(`/auto_backup/${id}/stored`)
}

/** Removes one stored run, its archive and its key file. */
export function deleteStoredBackup(id: number, key: string) {
  return http.delete(`/auto_backup/${id}/stored`, { params: { key } })
}

/** Fetches one stored run from its plugin backend and restores it. */
export function restoreStoredBackup(id: number, options: RestoreStoredOptions): Promise<RestoreResponse> {
  return http.post(`/auto_backup/${id}/stored/restore`, options)
}

// Auto backup CRUD API
export const autoBackup = useCurdApi<AutoBackup>('/auto_backup')

export default backup
