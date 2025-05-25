import type { ModelBase } from '@/api/curd'
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

/**
 * Interface for auto backup configuration
 */
export interface AutoBackup extends ModelBase {
  name: string
  backup_type: 'nginx_config' | 'nginx_ui_config' | 'both_config' | 'custom_dir'
  storage_type: 'local' | 's3'
  backup_path?: string
  storage_path: string
  cron_expression: string
  enabled: boolean
  last_backup_time?: string
  last_backup_status: 'pending' | 'success' | 'failed'
  last_backup_error?: string
  s3_endpoint?: string
  s3_access_key_id?: string
  s3_secret_access_key?: string
  s3_bucket?: string
  s3_region?: string
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
  restoreBackup(options: RestoreOptions) {
    const formData = new FormData()
    formData.append('backup_file', options.backup_file)
    formData.append('security_token', options.security_token)
    formData.append('restore_nginx', options.restore_nginx.toString())
    formData.append('restore_nginx_ui', options.restore_nginx_ui.toString())
    formData.append('verify_hash', options.verify_hash.toString())

    return http.post('/restore', formData, {
      headers: {
        'Content-Type': 'multipart/form-data;charset=UTF-8',
      },
      crypto: true,
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

// Auto backup CRUD API
export const autoBackup = useCurdApi<AutoBackup>('/auto_backup')

export default backup
