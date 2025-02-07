import http from '@/lib/http'

export interface OTPGenerateSecretResponse {
  secret: string
  url: string
}

const otp = {
  generate_secret(): Promise<OTPGenerateSecretResponse> {
    return http.get('/otp_secret')
  },
  enroll_otp(secret: string, passcode: string): Promise<{ recovery_code: string }> {
    return http.post('/otp_enroll', { secret, passcode })
  },
  reset(recovery_code: string) {
    return http.post('/otp_reset', { recovery_code })
  },
}

export default otp
