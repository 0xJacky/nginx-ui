import type { AuthenticationResponseJSON } from '@simplewebauthn/browser'
import { http } from '@uozi-admin/request'

export interface TwoFAStatus {
  required?: boolean
  policy_source?: 'optional' | 'user' | 'global'
  pending?: boolean
  enabled: boolean
  otp_status: boolean
  passkey_status: boolean
  recovery_codes_generated: boolean
  recovery_codes_viewed?: boolean
  recovery_codes_migration_required: boolean
}

export interface SecureSessionByOTPResponse {
  session_id: string
  // Seconds the backend keeps the session valid.
  session_ttl?: number
  used_legacy_recovery_code?: boolean
}

const twoFA = {
  status(): Promise<TwoFAStatus> {
    return http.get('/2fa_status', { skipNodeProxy: true })
  },
  start_secure_session_by_otp(passcode: string, recovery_code: string): Promise<SecureSessionByOTPResponse> {
    return http.post('/2fa_secure_session/otp', {
      otp: passcode,
      recovery_code,
    }, { skipNodeProxy: true })
  },
  secure_session_status(): Promise<{ status: boolean }> {
    return http.get('/2fa_secure_session/status', { skipNodeProxy: true })
  },
  begin_start_secure_session_by_passkey() {
    return http.get('/2fa_secure_session/passkey', { skipNodeProxy: true })
  },
  finish_start_secure_session_by_passkey(data: { session_id: string, options: AuthenticationResponseJSON }): Promise<{
    session_id: string
    session_ttl?: number
  }> {
    return http.post('/2fa_secure_session/passkey', data.options, {
      skipNodeProxy: true,
      headers: {
        'X-Passkey-Session-Id': data.session_id,
      },
    })
  },
}

export default twoFA
