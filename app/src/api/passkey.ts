import type { RegistrationResponseJSON } from '@simplewebauthn/browser'
import type { ModelBase } from '@/api/curd'
import { http } from '@uozi-admin/request'

export interface Passkey extends ModelBase {
  name: string
  user_id: string
  raw_id: string
  last_used_at: number
}

const passkey = {
  begin_registration(password: string) {
    return http.get('/begin_passkey_register', {
      skipNodeProxy: true,
      headers: {
        'X-Current-Password': password,
      },
    })
  },
  finish_registration(attestationResponse: RegistrationResponseJSON, passkeyName: string) {
    return http.post('/finish_passkey_register', attestationResponse, {
      skipNodeProxy: true,
      params: {
        name: passkeyName,
      },
    })
  },
  getList() {
    return http.get('/passkeys', { skipNodeProxy: true })
  },
  update(passkeyId: number, data: Passkey) {
    return http.post(`/passkeys/${passkeyId}`, data, { skipNodeProxy: true })
  },
  remove(passkeyId: number) {
    return http.delete(`/passkeys/${passkeyId}`, { skipNodeProxy: true })
  },
  get_config_status(): Promise<{ status: boolean }> {
    return http.get('/passkeys/config', { skipNodeProxy: true })
  },
}

export default passkey
