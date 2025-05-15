import type { ModelBase } from '@/api/curd'
import { useCurdApi } from '@uozi-admin/request'

export interface User extends ModelBase {
  name: string
  password: string
  enabled_2fa: boolean
  status: boolean
}

const user = useCurdApi<User>('/users')

export default user
