import type { ModelBase } from '@/api/curd'
import { useCurdApi } from '@uozi-admin/request'

export interface ExternalNotify extends ModelBase {
  type: string
  config: Record<string, string>
}

const baseUrl = '/external_notifies'

const externalNotify = useCurdApi<ExternalNotify>(baseUrl)

export default externalNotify
