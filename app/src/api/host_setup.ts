import { http } from '@uozi-admin/request'

export interface SetupParams {
  host_address: string
  host_user: string
  use_host_gateway?: boolean
  service_manager?: 'systemd' | 'launchd'
  systemd_unit?: string
  systemctl_path?: string
  launchd_service?: string
  launchctl_path?: string
  nginx_sbin_path?: string
  host_config_dir?: string
  host_log_dir?: string
  pid_path?: string
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

export interface NginxDiscovery {
  version: string
  executable_path: string
  prefix?: string
  config_path?: string
  config_dir?: string
  pid_path?: string
  access_log_path?: string
  error_log_path?: string
  log_dir?: string
  homebrew_prefix?: string
  document_root?: string
}

export interface KeypairResponse {
  public_key: string
  private_key?: string
}

export type HostKeyStatus = 'trusted' | 'unknown_host' | 'new_algorithm' | 'changed' | 'stale'

export interface HostKeyScanItem {
  algorithm: string
  public_key: string
  fingerprint: string
  existing_fingerprint?: string
  status: HostKeyStatus
}

export interface KnownHostsPersistence {
  path: string
  recommended: boolean
  warning?: string
}

export interface HostKeyScanResult {
  host_address: string
  known_hosts_path: string
  keys: HostKeyScanItem[]
  stale_keys: HostKeyScanItem[]
  persistence: KnownHostsPersistence
}

export interface HostKeyScanRequest {
  host_address: string
  keyscan_output?: string
}

export interface HostKeyTrustRequest {
  host_address: string
  algorithm: string
  fingerprint: string
  public_key: string
  confirmed: boolean
}

export interface HostKeyReplaceRequest {
  host_address: string
  algorithm: string
  old_fingerprint: string
  new_fingerprint: string
  public_key: string
  confirmed: boolean
}

export interface HostKeyDeleteRequest {
  host_address: string
  algorithm: string
  fingerprint: string
  confirmed: boolean
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
  verify(params: SetupParams, skipNginxT = false): Promise<VerifyResult> {
    return http.post('/host/setup/verify', { ...params, skip_nginx_t: skipNginxT })
  },
  discover(params: SetupParams): Promise<NginxDiscovery> {
    return http.post('/host/setup/discover', params)
  },
  trustHostKey(hostAddress: string, fingerprint: string, publicKey: string): Promise<void> {
    return http.post('/host/setup/known-host', {
      host_address: hostAddress,
      fingerprint,
      public_key: publicKey,
    })
  },
  scanHostKeys(payload: HostKeyScanRequest): Promise<HostKeyScanResult> {
    return http.post('/host/setup/host-key/scan', payload)
  },
  trustScannedHostKey(payload: HostKeyTrustRequest): Promise<void> {
    return http.post('/host/setup/host-key/trust', payload)
  },
  replaceHostKey(payload: HostKeyReplaceRequest): Promise<void> {
    return http.post('/host/setup/host-key/replace', payload)
  },
  deleteHostKey(payload: HostKeyDeleteRequest): Promise<void> {
    return http.delete('/host/setup/host-key', { data: payload })
  },
}

export default hostSetup
