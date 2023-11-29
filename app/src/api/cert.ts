import type { ModelBase } from '@/api/curd'
import Curd from '@/api/curd'
import type { DnsCredential } from '@/api/dns_credential'

export interface Cert extends ModelBase {
  name: string
  domains: string[]
  filename: string
  ssl_certificate_path: string
  ssl_certificate_key_path: string
  auto_cert: number
  challenge_method: string
  dns_credential_id: number
  dns_credential?: DnsCredential
  log: string
}

export interface CertificateInfo {
  subject_name: string
  issuer_name: string
  not_after: string
  not_before: string
}

const cert: Curd<Cert> = new Curd('/cert')

export default cert
