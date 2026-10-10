import type { AuthenticationResponseJSON, PublicKeyCredentialRequestOptionsJSON } from '@simplewebauthn/browser'
import { http } from '@uozi-admin/request'
import { useUserStore } from '@/pinia'

const { logout } = useUserStore()

export interface AuthResponse {
  message: string
  token: string
  code: number
  error: string
  secure_session_id: string
  secure_session_ttl?: number
  mfa_stage?: 'setup' | 'verify'
  pre_auth_id?: string
  options?: {
    publicKey: PublicKeyCredentialRequestOptionsJSON
  }
}

const auth = {
  async login(name: string, password: string, otp: string, recoveryCode: string): Promise<AuthResponse> {
    return http.post('/login', {
      name,
      password,
      otp,
      recovery_code: recoveryCode,
    }, { crypto: true, skipNodeProxy: true })
  },
  casdoor_login(code?: string, state?: string): Promise<AuthResponse> {
    return http.post('/casdoor_callback', { code, state }, { skipNodeProxy: true })
  },
  oidc_login(code?: string, state?: string): Promise<AuthResponse> {
    return http.post('/oidc_callback', { code, state }, { skipNodeProxy: true })
  },
  async logout() {
    return http.delete('/logout', { skipNodeProxy: true }).then(async () => {
      logout()
    })
  },
  async get_casdoor_uri(): Promise<{ uri: string }> {
    return http.get('/casdoor_uri', { skipNodeProxy: true })
  },
  async get_oidc_uri(): Promise<{ uri: string }> {
    return http.get('/oidc_uri', { skipNodeProxy: true })
  },
  begin_passkey_login() {
    return http.get('/begin_passkey_login', { skipNodeProxy: true })
  },
  finish_passkey_login(data: { session_id: string, options: AuthenticationResponseJSON }) {
    return http.post('/finish_passkey_login', data.options, {
      skipNodeProxy: true,
      headers: {
        'X-Passkey-Session-Id': data.session_id,
      },
    })
  },
  finish_passkey_pre_auth(data: { pre_auth_id: string, options: AuthenticationResponseJSON }): Promise<AuthResponse> {
    return http.post('/finish_passkey_pre_auth', data.options, {
      skipNodeProxy: true,
      headers: {
        'X-Passkey-Pre-Auth-ID': data.pre_auth_id,
      },
    })
  },
}

export default auth
