import Curd from '@/api/curd'
import http from '@/lib/http'
import type { ChatComplicationMessage } from '@/api/openai'
import type { CertificateInfo } from '@/api/cert'
import type { NgxConfig } from '@/api/ngx'
import type { PrivateKeyType } from '@/constants'

export interface Site {
  modified_at: string
  advanced: boolean
  enabled: boolean
  name: string
  config: string
  auto_cert: boolean
  chatgpt_messages: ChatComplicationMessage[]
  tokenized?: NgxConfig
  cert_info?: {
    [key: number]: CertificateInfo
  }
}

export interface AutoCertRequest {
  dns_credential_id: number | null
  challenge_method: string
  domains: string[]
  key_type: PrivateKeyType
}

class Domain extends Curd<Site> {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  enable(name: string, config?: any) {
    return http.post(`${this.baseUrl}/${name}/enable`, undefined, config)
  }

  disable(name: string) {
    return http.post(`${this.baseUrl}/${name}/disable`)
  }

  get_template() {
    return http.get('template')
  }

  add_auto_cert(domain: string, data: AutoCertRequest) {
    return http.post(`auto_cert/${domain}`, data)
  }

  remove_auto_cert(domain: string) {
    return http.delete(`auto_cert/${domain}`)
  }

  duplicate(name: string, data: { name: string }): Promise<{ dst: string }> {
    return http.post(`${this.baseUrl}/${name}/duplicate`, data)
  }

  advance_mode(name: string, data: { advanced: boolean }) {
    return http.post(`${this.baseUrl}/${name}/advance`, data)
  }
}

const domain = new Domain('/domain')

export default domain
