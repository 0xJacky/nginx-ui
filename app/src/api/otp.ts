import type { RecoveryCodesResponse } from '@/api/recovery'
import { http } from '@uozi-admin/request'

export interface OTPGenerateSecretResponse {
  secret: string
  url: string
}

const otp = {
  generate_secret(): Promise<OTPGenerateSecretResponse> {
    return http.get('/otp_secret', { skipNodeProxy: true })
  },
  enroll_otp(secret: string, passcode: string, password: string, replace = false): Promise<RecoveryCodesResponse> {
    return http.post('/otp_enroll', { secret, passcode, password, replace }, { skipNodeProxy: true })
  },
  reset() {
    return http.get('/otp_reset', { skipNodeProxy: true })
  },
}

export default otp
