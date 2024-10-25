import type { ModelBase } from '@/api/curd'
import Curd from '@/api/curd'

export interface User extends ModelBase {
  name: string
  password: string
}

const user: Curd<User> = new Curd('users')

export default user
