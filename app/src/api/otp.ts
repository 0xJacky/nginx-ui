import http from '@/lib/http'

export interface OTPGenerateSecretResponse {
  secret: string
  qr_code: string
}

const otp = {
  status(): Promise<{ status: boolean }> {
    return http.get('/otp_status')
  },
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
