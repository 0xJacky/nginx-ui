import type { ModelBase } from '@/api/curd'
import { extendCurdApi, http, useCurdApi } from '@uozi-admin/request'

export interface AcmeUser extends ModelBase {
  name: string
  email: string
  ca_dir: string
  registration: { body?: { status: string } }
}

const baseUrl = '/acme_users'

const acme_user = extendCurdApi(useCurdApi<AcmeUser>(baseUrl), {
  register: (id: number) => http.post(`${baseUrl}/${id}/register`),
})

export default acme_user
