import http from '@/lib/http'

export interface RecoveryCode {
  code: string
  used_time?: number
}

export interface RecoveryCodes {
  codes: RecoveryCode[]
  last_viewed?: number
  last_downloaded?: number
}

export interface RecoveryCodesResponse extends RecoveryCodes {
  message: string
}

const recovery = {
  generate(): Promise<RecoveryCodesResponse> {
    return http.get('/recovery_codes_generate')
  },
  view(): Promise<RecoveryCodesResponse> {
    return http.get('/recovery_codes')
  },
}

export default recovery
