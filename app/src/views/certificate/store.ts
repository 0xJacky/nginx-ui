import type { Cert } from '@/api/cert'
import cert from '@/api/cert'

export const useCertStore = defineStore('cert', () => {
  const data = ref<Cert>({} as Cert)

  async function save() {
    const previousData = data.value
    const r = data.value.id
      ? await cert.updateItem(data.value.id, data.value)
      : await cert.createItem(data.value)
    data.value = {
      ...previousData,
      ...r,
    }
  }

  return {
    data,
    save,
  }
})
