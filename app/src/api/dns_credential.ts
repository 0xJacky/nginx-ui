import type { DNSProvider } from '@/api/auto_cert'
import type { ModelBase } from '@/api/curd'
import { useCurdApi } from '@uozi-admin/request'

export interface DnsCredential extends ModelBase {
  name: string
  config?: DNSProvider
  provider: string
  provider_code?: string
  code: string
  configuration: DNSProvider['configuration']
}

const baseUrl = '/dns_credentials'

const dns_credential = useCurdApi<DnsCredential>(baseUrl)

export default dns_credential
