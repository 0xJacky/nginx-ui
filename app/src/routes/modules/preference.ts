import type { RouteRecordRaw } from 'vue-router'
import { SettingOutlined } from '@ant-design/icons-vue'

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
