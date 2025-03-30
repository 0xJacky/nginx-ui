import http from '@/lib/http'

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

const backup = {
  /**
   * Create and download a backup of nginx-ui and nginx configurations
   * Use http module with returnFullResponse option to access headers
   */
  createBackup() {
    return http.get('/system/backup', {
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

    return http.post('/system/backup/restore', formData, {
      headers: {
        'Content-Type': 'multipart/form-data;charset=UTF-8',
      },
      crypto: true,
    })
  },
}

export default backup
