import http from '@/lib/http'

export interface DNSProvider {
  name: string
  code: string
  configuration: {
    credentials: {
      [key: string]: string
    }
    additional: {
      [key: string]: string
    }
  }
  links: {
    api: string
    go_client: string
  }
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
