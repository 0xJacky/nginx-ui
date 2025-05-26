import type { ModelBase } from '@/api/curd'
import { extendCurdApi, http, useCurdApi } from '@uozi-admin/request'

export interface User extends ModelBase {
  name: string
  password: string
  enabled_2fa: boolean
  status: boolean
}

const user = extendCurdApi(useCurdApi<User>('/users'), {
  getCurrentUser: () => {
    return http.get('/user')
  },
  updateCurrentUser: (data: User) => {
    return http.post('/user', data)
  },
  updateCurrentUserPassword: (data: { old_password: string, new_password: string }) => {
    return http.post('/user/password', data)
  },
})

export default user
