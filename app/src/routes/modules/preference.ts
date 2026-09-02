import type { RouteRecordRaw } from 'vue-router'
import { SettingOutlined } from '@antdv-next/icons'

export const preferenceRoutes: RouteRecordRaw[] = [
  {
    path: 'preference',
    name: 'Preference',
    component: () => import('@/views/preference/Preference.vue'),
    meta: {
      name: () => $gettext('Preference'),
      icon: SettingOutlined,
    },
  },
]
