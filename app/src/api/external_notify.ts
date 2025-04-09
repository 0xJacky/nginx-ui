import type { ModelBase } from '@/api/curd'
import Curd from '@/api/curd'

export interface ExternalNotify extends ModelBase {
  type: string
  config: Record<string, string>
}

class ExternalNotifyCurd extends Curd<ExternalNotify> {
  constructor() {
    super('/external_notifies')
  }
}

const externalNotify: ExternalNotifyCurd = new ExternalNotifyCurd()

export default externalNotify
