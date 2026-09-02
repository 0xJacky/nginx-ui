import type { RouteRecordRaw } from 'vue-router'
import { BellOutlined } from '@antdv-next/icons'

export const notificationsRoutes: RouteRecordRaw[] = [
  {
    path: 'notifications',
    name: 'Notifications',
    component: () => import('@/views/notification/Notification.vue'),
    meta: {
      name: () => $gettext('Notifications'),
      icon: BellOutlined,
    },
  },
]
