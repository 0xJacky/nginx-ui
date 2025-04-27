import type { NginxStatus } from '@/constants'
import { defineStore } from 'pinia'

interface ProcessingStatus {
  index_scanning: boolean
  auto_cert_processing: boolean
}

type NginxStatusType = NginxStatus.Reloading | NginxStatus.Restarting | NginxStatus.Running | NginxStatus.Stopped

export const useGlobalStore = defineStore('global', () => {
  const nginxStatus: Ref<NginxStatusType> = ref(0)

  const processingStatus = ref<ProcessingStatus>({
    index_scanning: false,
    auto_cert_processing: false,
  })
  return {
    nginxStatus,
    processingStatus,
  }
})
