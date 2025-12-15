import type { AuthenticationResponseJSON } from '@simplewebauthn/browser'
import { http } from '@uozi-admin/request'
import { useUserStore } from '@/pinia'

const { login, logout } = useUserStore()

export interface AuthResponse {
  message: string
  token: string
  short_token: string
  code: number
  error: string
  secure_session_id: string
}

const auth = {
  async login(name: string, password: string, otp: string, recoveryCode: string): Promise<AuthResponse> {
    return http.post('/login', {
      name,
      password,
      otp,
      recovery_code: recoveryCode,
    }, { crypto: true })
  },
  async casdoor_login(code?: string, state?: string) {
    await http.post('/casdoor_callback', {
      code,
      state,
    })
      .then((r: AuthResponse) => {
        login(r.token, r.short_token)
      })
  },
  async oidc_login(code?: string, state?: string) {
    await http.post('/oidc_callback', {
      code,
      state,
    })
      .then((r: AuthResponse) => {
        login(r.token, r.short_token)
      })
  },
  async logout() {
    return http.delete('/logout').then(async () => {
      logout()
    })
  },
  async get_casdoor_uri(): Promise<{ uri: string }> {
    return http.get('/casdoor_uri')
  },
  async get_oidc_uri(): Promise<{ uri: string }> {
    return http.get('/oidc_uri')
  },
  begin_passkey_login() {
    return http.get('/begin_passkey_login')
  },
  finish_passkey_login(data: { session_id: string, options: AuthenticationResponseJSON }) {
    return http.post('/finish_passkey_login', data.options, {
      headers: {
        'X-Passkey-Session-Id': data.session_id,
      },
    })
  },
}

export default auth
