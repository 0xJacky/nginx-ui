import type { AutoCertChallengeMethod } from './auto_cert'
import type { AcmeUser } from '@/api/acme_user'
import type { ModelBase } from '@/api/curd'
import type { DnsCredential } from '@/api/dns_credential'
import type { PrivateKeyType } from '@/constants'
import { extendCurdApi, http, useCurdApi } from '@uozi-admin/request'
import { normalizePrivateKeyType, PrivateKeyTypeEnum } from '@/constants'

export const CertStatus = {
  Pending: 'pending',
  Success: 'success',
  Failure: 'failure',
} as const

export type CertStatusType = '' | typeof CertStatus[keyof typeof CertStatus]

export interface SelfSignedCertConfig {
  ip_addresses: string[]
  validity_days: number
}

export interface Cert extends ModelBase {
  name: string
  domains: string[]
  filename: string
  ssl_certificate_path: string
  ssl_certificate: string
  ssl_certificate_key_path: string
  ssl_certificate_key: string
  auto_cert: number
  challenge_method: keyof typeof AutoCertChallengeMethod
  dns_credential_id: number
  dns_credential?: DnsCredential
  acme_user_id: number
  acme_user?: AcmeUser
  key_type: string
  log: string
  certificate_info: CertificateInfo
  sync_node_ids: number[]
  revoke_old: boolean
  status: CertStatusType
  last_error: string
  last_attempt_at: string | null
  self_signed_config?: SelfSignedCertConfig
}

export interface CertificateInfo {
  subject_name: string
  issuer_name: string
  not_after: string
  not_before: string
}

export interface CertificateResult {
  ssl_certificate: string
  ssl_certificate_key: string
  key_type: PrivateKeyType
}

export interface SelfSignedCertPayload {
  name: string
  domains: string[]
  ip_addresses: string[]
  key_type: string
  validity_days: number
  sync_node_ids?: number[]
}

// toSelfSignedPayload maps a persisted Cert to an editable self-signed payload.
export function toSelfSignedPayload(c: Cert): SelfSignedCertPayload {
  const domains = c.domains?.length ? [...c.domains] : ['']
  const ipAddresses = c.self_signed_config?.ip_addresses?.length
    ? [...c.self_signed_config.ip_addresses]
    : ['']
  // Backend stores key_type in its canonical form (EC256, RSA2048…); the
  // form ASelect expects the legacy keys (P256, 2048…). Normalize so the
  // option highlights correctly when editing an existing self-signed cert.
  const keyType = normalizePrivateKeyType(c.key_type) || PrivateKeyTypeEnum.P256
  return {
    name: c.name ?? '',
    domains,
    ip_addresses: ipAddresses,
    key_type: keyType,
    validity_days: c.self_signed_config?.validity_days || 365,
    sync_node_ids: [...(c.sync_node_ids ?? [])],
  }
}

const cert = extendCurdApi(useCurdApi<Cert>('/certs'), {
  generate_self_signed(payload: SelfSignedCertPayload): Promise<Cert> {
    return http.post('/self_signed_cert', payload)
  },
  modify_self_signed(id: number, payload: SelfSignedCertPayload): Promise<Cert> {
    return http.post(`/self_signed_cert/${id}`, payload)
  },
})

export default cert
