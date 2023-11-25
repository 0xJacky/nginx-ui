import http from '@/lib/http'

const auto_cert = {
  get_dns_providers() {
    return http.get('/auto_cert/dns/providers')
  },

  get_dns_provider(code: string) {
    return http.get('/auto_cert/dns/provider/' + code)
  }
}

export default auto_cert
