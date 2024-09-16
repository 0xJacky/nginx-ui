import type { AuthenticationResponseJSON } from '@simplewebauthn/types'
import http from '@/lib/http'

export interface TwoFAStatusResponse {
  enabled: boolean
  otp_status: boolean
  passkey_status: boolean
}

const twoFA = {
  status(): Promise<TwoFAStatusResponse> {
    return http.get('/2fa_status')
  },
  start_secure_session_by_otp(passcode: string, recovery_code: string): Promise<{ session_id: string }> {
    return http.post('/2fa_secure_session/otp', {
      otp: passcode,
      recovery_code,
    })
  },
  secure_session_status(): Promise<{ status: boolean }> {
    return http.get('/2fa_secure_session/status')
  },
  begin_start_secure_session_by_passkey() {
    return http.get('/2fa_secure_session/passkey')
  },
  finish_start_secure_session_by_passkey(data: { session_id: string; options: AuthenticationResponseJSON }): Promise<{
    session_id: string
  }> {
    return http.post('/2fa_secure_session/passkey', data.options, {
      headers: {
        'X-Passkey-Session-Id': data.session_id,
      },
    })
  },
}

export default twoFA
