import type { RegistrationResponseJSON } from '@simplewebauthn/types'
import http from '@/lib/http'
import type { ModelBase } from '@/api/curd'

export interface Passkey extends ModelBase {
  name: string
  user_id: string
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
  get_passkey_enabled() {
    return http.get('/passkey_enabled')
  },
}

export default passkey
