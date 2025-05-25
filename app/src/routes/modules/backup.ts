import type { RouteRecordRaw } from 'vue-router'
import { ClockCircleOutlined } from '@ant-design/icons-vue'

export const backupRoutes: RouteRecordRaw[] = [
  {
    path: 'backup',
    name: 'Backup',
    component: () => import('@/layouts/BaseRouterView.vue'),
    meta: {
      icon: ClockCircleOutlined,
      name: () => $gettext('Backup'),
    },
    children: [
      {
        path: 'backup-and-restore',
        name: 'BackupAndRestore',
        component: () => import('@/views/backup/index.vue'),
        meta: {
          name: () => $gettext('Backup'),
        },
      },
      {
        path: 'auto-backup',
        name: 'AutoBackup',
        component: () => import('@/views/backup/AutoBackup/AutoBackup.vue'),
        meta: {
          name: () => $gettext('Auto Backup'),
        },
      },
    ],
  },
]
