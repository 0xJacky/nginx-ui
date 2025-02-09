import type { RecoveryCodesResponse } from '@/api/recovery'
import http from '@/lib/http'

export interface OTPGenerateSecretResponse {
  secret: string
  url: string
}

const otp = {
  generate_secret(): Promise<OTPGenerateSecretResponse> {
    return http.get('/otp_secret')
  },
  enroll_otp(secret: string, passcode: string): Promise<RecoveryCodesResponse> {
    return http.post('/otp_enroll', { secret, passcode })
  },
  reset() {
    return http.get('/otp_reset')
  },
}

export default otp
