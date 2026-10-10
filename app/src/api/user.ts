import type { ModelBase } from '@/api/curd'
import { extendCurdApi, http, useCurdApi } from '@uozi-admin/request'

export interface User extends ModelBase {
  name: string
  password: string
  enabled_2fa: boolean
  mfa_required: boolean
  mfa_policy_source: 'optional' | 'user' | 'global'
  mfa_pending: boolean
  status: boolean
  language: string
}

const user = extendCurdApi(useCurdApi<User>('/users'), {
  resetMFA: (id: number) => http.post(`/users/${id}/mfa/reset`),
  getCurrentUser: () => {
    return http.get('/user', { skipNodeProxy: true })
  },
  updateCurrentUser: (data: Partial<User>) => {
    return http.post('/user', data, { skipNodeProxy: true })
  },
  updateCurrentUserPassword: (data: { old_password: string, new_password: string }) => {
    return http.post('/user/password', data, { skipNodeProxy: true })
  },
  updateCurrentUserLanguage: (data: { language: string }) => {
    return http.post('/user/language', data, { skipNodeProxy: true })
  },
  fetchShortToken: (): Promise<{ short_token: string }> => {
    return http.post('/token/short', {}, { skipNodeProxy: true })
  },
})

export default user
