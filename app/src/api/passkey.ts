import type { RegistrationResponseJSON } from '@simplewebauthn/browser'
import type { ModelBase } from '@/api/curd'
import http from '@/lib/http'

export interface Passkey extends ModelBase {
  name: string
  user_id: string
  raw_id: string
}

const passkey = {
  begin_registration() {
    return http.get('/begin_passkey_register')
  },
  finish_registration(attestationResponse: RegistrationResponseJSON, passkeyName: string) {
    return http.post('/finish_passkey_register', attestationResponse, {
      params: {
        name: passkeyName,
      },
    })
  },
  get_list() {
    return http.get('/passkeys')
  },
  update(passkeyId: number, data: Passkey) {
    return http.post(`/passkeys/${passkeyId}`, data)
  },
  remove(passkeyId: number) {
    return http.delete(`/passkeys/${passkeyId}`)
  },
  get_config_status(): Promise<{ status: boolean }> {
    return http.get('/passkeys/config')
  },
}

export default passkey
