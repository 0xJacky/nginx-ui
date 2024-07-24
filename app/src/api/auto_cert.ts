import http from '@/lib/http'

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
  code?: string
  dns_credential_id?: number | null
  challenge_method?: string
  configuration?: DNSProvider['configuration']
  key_type: string
  acme_user_id?: number
  provider?: string
  must_staple?: boolean
  lego_disable_cname_support?: boolean
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
