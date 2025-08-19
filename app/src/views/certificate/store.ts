import type { Cert } from '@/api/cert'
import { message } from 'ant-design-vue'
import cert from '@/api/cert'

export const useCertStore = defineStore('cert', () => {
  const data = ref<Cert>({} as Cert)

  async function save() {
    const r = data.value.id
      ? await cert.updateItem(data.value.id, data.value)
      : await cert.createItem(data.value)
    data.value = r
    message.success($gettext('Save successfully'))
  }

  return {
    data,
    save,
  }
})
