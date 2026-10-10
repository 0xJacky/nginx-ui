import type { AuthenticationResponseJSON, PublicKeyCredentialCreationOptionsJSON, PublicKeyCredentialRequestOptionsJSON, RegistrationResponseJSON } from '@simplewebauthn/browser'
import type { AuthResponse } from '@/api/auth'
import type { OTPGenerateSecretResponse } from '@/api/otp'
import type { RecoveryCodes } from '@/api/recovery'
import { http } from '@uozi-admin/request'

export interface MFAPreAuthStatus {
  mfa_stage: 'setup' | 'verify'
  otp_status: boolean
  passkey_status: boolean
  passkey_available: boolean
  recovery_codes_generated: boolean
  expires_at: number
}

export interface MFAAuthResponse extends AuthResponse {
  recovery_codes?: RecoveryCodes
}

export function useMFAPreAuth(id: string) {
  const config = { skipNodeProxy: true, skipAuthRedirect: true, skipErrHandling: true, headers: { 'X-MFA-Pre-Auth-ID': id } }
  const path = '/mfa/pre_auth'
  return {
    status: (): Promise<MFAPreAuthStatus> => http.get(`${path}/status`, config),
    beginTOTP: (): Promise<OTPGenerateSecretResponse> => http.post(`${path}/totp/begin`, {}, config),
    finishTOTP: (passcode: string): Promise<MFAAuthResponse> => http.post(`${path}/totp/finish`, { passcode }, config),
    verifyOTP: (otp: string, recovery_code: string): Promise<MFAAuthResponse> => http.post(`${path}/otp`, { otp, recovery_code }, config),
    beginPasskey: (): Promise<PublicKeyCredentialCreationOptionsJSON | PublicKeyCredentialRequestOptionsJSON> => http.post(`${path}/passkey/begin`, {}, config),
    finishPasskey: (response: RegistrationResponseJSON | AuthenticationResponseJSON): Promise<MFAAuthResponse> => http.post(`${path}/passkey/finish`, response, config),
  }
}
