import type { NginxStatus } from '@/constants'
import { defineStore } from 'pinia'

export const useGlobalStore = defineStore('global', () => {
  const nginxStatus:
  Ref<NginxStatus.Reloading | NginxStatus.Restarting | NginxStatus.Running | NginxStatus.Stopped>
      = ref(0)

  return {
    nginxStatus,
  }
})
