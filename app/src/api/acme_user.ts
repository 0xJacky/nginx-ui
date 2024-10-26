import type { ModelBase } from '@/api/curd'
import Curd from '@/api/curd'
import http from '@/lib/http'

export interface AcmeUser extends ModelBase {
  name: string
  email: string
  ca_dir: string
  registration: { body?: { status: string } }
}

class ACMEUserCurd extends Curd<AcmeUser> {
  constructor() {
    super('acme_users')
  }

  public async register(id: number) {
    return http.post(`${this.baseUrl}/${id}/register`)
  }
}

const acme_user = new ACMEUserCurd()

export default acme_user
