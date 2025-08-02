import { http } from '@uozi-admin/request'

export const AutoCertChallengeMethod = {
  http01: 'http01',
  dns01: 'dns01',
} as const

export interface DNSProvider {
  name?: string
  code?: string
  provider?: string
  configuration: {
    credentials: Record<string, string>
    additional: Record<string, string>
  }
  links?: {
    api: string
    go_client: string
  }
}

export interface AutoCertOptions {
  name?: string
  domains: string[]
  ip_address?: string
  code?: string
  dns_credential_id?: number | null
  challenge_method: keyof typeof AutoCertChallengeMethod
  configuration?: DNSProvider['configuration']
  key_type: string
  acme_user_id?: number
  provider?: string
  must_staple?: boolean
  lego_disable_cname_support?: boolean
  revoke_old?: boolean
}

const auto_cert = {
  get_dns_providers(): Promise<DNSProvider[]> {
    return http.get('/certificate/dns_providers')
  },

  get_dns_provider(code: string): Promise<DNSProvider> {
    return http.get(`/certificate/dns_provider/${code}`)
  },
}

export default auto_cert
