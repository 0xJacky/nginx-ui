import { http } from '@uozi-admin/request'

export interface SetupParams {
  host_address: string
  host_user: string
  use_host_gateway?: boolean
  systemd_unit?: string
  systemctl_path?: string
  nginx_sbin_path?: string
  host_config_dir?: string
  host_log_dir?: string
  use_generated_key?: boolean
  public_key_open_ssh?: string
}

export interface RenderedSnippets {
  compose_snippet: string
  compose_override: string
  docker_run: string
  authorized_keys: string
  sudoers: string
  acl_commands: string
}

export interface StepOutcome {
  ok: boolean
  level?: 'success' | 'warning' | 'error'
  detail: string
  remediation?: string
}

export interface VerifyResult {
  steps: Record<string, StepOutcome>
}

export interface KeypairResponse {
  public_key: string
  private_key?: string
}

const hostSetup = {
  preview(params?: SetupParams): Promise<RenderedSnippets> {
    return http.post('/host/setup/preview', params ?? {})
  },
  generateKeypair(): Promise<KeypairResponse> {
    return http.post('/host/setup/keypair')
  },
  getPublicKey(): Promise<{ public_key: string }> {
    return http.get('/host/setup/publickey')
  },
  deleteKeypair(): Promise<void> {
    return http.delete('/host/setup/keypair')
  },
  verify(skipNginxT = false): Promise<VerifyResult> {
    return http.post('/host/setup/verify', { skip_nginx_t: skipNginxT })
  },
  trustHostKey(hostAddress: string, fingerprint: string, publicKey: string): Promise<void> {
    return http.post('/host/setup/known-host', {
      host_address: hostAddress,
      fingerprint,
      public_key: publicKey,
    })
  },
}

export default hostSetup
