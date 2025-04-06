import type { CertificateInfo } from '@/api/cert'
import type { ModelBase } from '@/api/curd'
import type { EnvGroup } from '@/api/env_group'
import type { NgxConfig } from '@/api/ngx'
import type { ChatComplicationMessage } from '@/api/openai'
import type { PrivateKeyType } from '@/constants'
import Curd from '@/api/curd'
import http from '@/lib/http'

export interface Site extends ModelBase {
  modified_at: string
  path: string
  advanced: boolean
  enabled: boolean
  name: string
  filepath: string
  config: string
  auto_cert: boolean
  chatgpt_messages: ChatComplicationMessage[]
  tokenized?: NgxConfig
  cert_info?: Record<number, CertificateInfo[]>
  env_group_id: number
  env_group?: EnvGroup
  sync_node_ids: number[]
  urls?: string[]
}

export interface AutoCertRequest {
  dns_credential_id: number | null
  challenge_method: string
  domains: string[]
  key_type: PrivateKeyType
}

class SiteCurd extends Curd<Site> {
  // eslint-disable-next-line ts/no-explicit-any
  enable(name: string, config?: any) {
    return http.post(`${this.baseUrl}/${encodeURIComponent(name)}/enable`, undefined, config)
  }

  disable(name: string) {
    return http.post(`${this.baseUrl}/${name}/disable`)
  }

  rename(oldName: string, newName: string) {
    return http.post(`${this.baseUrl}/${encodeURIComponent(oldName)}/rename`, { new_name: newName })
  }

  get_default_template() {
    return http.get('default_site_template')
  }

  add_auto_cert(domain: string, data: AutoCertRequest) {
    return http.post(`auto_cert/${encodeURIComponent(domain)}`, data)
  }

  remove_auto_cert(domain: string) {
    return http.delete(`auto_cert/${encodeURIComponent(domain)}`)
  }

  duplicate(name: string, data: { name: string }): Promise<{ dst: string }> {
    return http.post(`${this.baseUrl}/${encodeURIComponent(name)}/duplicate`, data)
  }

  advance_mode(name: string, data: { advanced: boolean }) {
    return http.post(`${this.baseUrl}/${encodeURIComponent(name)}/advance`, data)
  }
}

const site = new SiteCurd('/sites')

export default site
