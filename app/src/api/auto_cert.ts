import http from '@/lib/http'

export interface DNSProvider {
  name?: string
  code?: string
  provider?: string
  configuration: {
    credentials: {
      [key: string]: string
    }
    additional: {
      [key: string]: string
    }
  }
  links?: {
    api: string
    go_client: string
  }
}
export interface DnsChallenge extends DNSProvider {
  dns_credential_id: number | null
  challenge_method: string
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
