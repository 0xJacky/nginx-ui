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
  /** Set when the provider also supports DNS record management in DNS Domains. */
  record_management?: boolean
  /** Set when the provider can solve ACME DNS-01 challenges. */
  dns01?: boolean
  /** Identifier of the plugin contributing this provider, empty for built-ins. */
  plugin_id?: string
}

/** One challenge method the current installation can actually run. */
export interface ChallengeMethod {
  code: keyof typeof AutoCertChallengeMethod
  /** True for methods implemented by the core rather than by a plugin. */
  builtin?: boolean
  plugin_id?: string
}

export interface AutoCertOptions {
  name?: string
  domains: string[]
  code?: string
  dns_credential_id?: number | null
  challenge_method: keyof typeof AutoCertChallengeMethod
  profile?: string
  configuration?: DNSProvider['configuration']
  key_type: string
  acme_user_id?: number
  provider?: string
  provider_code?: string
  must_staple?: boolean
  enable_common_name?: boolean
  revoke_old?: boolean
  /** Free-form payload owned by the plugin that implements the challenge method. */
  challenge_config?: Record<string, unknown>
}

const auto_cert = {
  get_dns_providers(): Promise<DNSProvider[]> {
    return http.get('/certificate/dns_providers')
  },

  get_dns_provider(code: string): Promise<DNSProvider> {
    return http.get(`/certificate/dns_provider/${code}`)
  },

  get_challenge_methods(): Promise<ChallengeMethod[]> {
    return http.get('/certificate/challenge_methods')
  },
}

export default auto_cert
