import type { RouteRecordRaw } from 'vue-router'
import { InfoCircleOutlined } from '@ant-design/icons-vue'
import { useSettingsStore } from '@/pinia'

export const systemRoutes: RouteRecordRaw[] = [
  {
    path: 'system',
    name: 'System',
    redirect: 'system/about',
    meta: {
      name: () => $gettext('System'),
      icon: InfoCircleOutlined,
    },
    children: [{
      path: 'self_check',
      name: 'Self Check',
      component: () => import('@/views/system/SelfCheck.vue'),
      meta: {
        name: () => $gettext('Self Check'),
      },
    }, {
      path: 'upgrade',
      name: 'Upgrade',
      component: () => import('@/views/system/Upgrade.vue'),
      meta: {
        name: () => $gettext('Upgrade'),
        hiddenInSidebar: (): boolean => {
          const settings = useSettingsStore()

          return settings.is_remote
        },
      },
    }, {
      path: 'about',
      name: 'About',
      component: () => import('@/views/system/About.vue'),
      meta: {
        name: () => $gettext('About'),
      },
    }],
  },
]
