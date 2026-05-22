import type { AutoCertChallengeMethod } from './auto_cert'
import type { AcmeUser } from '@/api/acme_user'
import type { ModelBase } from '@/api/curd'
import type { DnsCredential } from '@/api/dns_credential'
import type { PrivateKeyType } from '@/constants'
import { useCurdApi } from '@uozi-admin/request'

export const CertStatus = {
  Pending: 'pending',
  Success: 'success',
  Failure: 'failure',
} as const

export type CertStatusType = '' | typeof CertStatus[keyof typeof CertStatus]

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
  last_attempt_at: string
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

const cert = useCurdApi<Cert>('/certs')

export default cert
