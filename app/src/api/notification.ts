import type { ModelBase } from '@/api/curd'
import Curd from '@/api/curd'
import http from '@/lib/http'

export interface Notification extends ModelBase {
  type: string
  title: string
  details: string
}

class NotificationCurd extends Curd<Notification> {
  public clear() {
    return http.delete(this.plural)
  }
}

const notification = new NotificationCurd('/notification')

export default notification
