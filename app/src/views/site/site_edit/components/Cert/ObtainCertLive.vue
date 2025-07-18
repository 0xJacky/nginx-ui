<script setup lang="ts">
import type { Ref } from 'vue'
import type { AutoCertOptions } from '@/api/auto_cert'
import type { CertificateResult } from '@/api/cert'
import websocket from '@/lib/websocket'
import { useSiteEditorStore } from '../SiteEditor/store'

const props = defineProps<{
  options: AutoCertOptions
}>()

const modalVisible = defineModel<boolean>('modalVisible')
const modalClosable = defineModel<boolean>('modalClosable')

const editorStore = useSiteEditorStore()
const { issuingCert } = storeToRefs(editorStore)

const progressStrokeColor = {
  from: '#108ee9',
  to: '#87d068',
}

const progressPercent = ref(0)
const progressStatus = ref('active') as Ref<'success' | 'active' | 'normal' | 'exception'>

const logContainer = useTemplateRef('logContainer')

function log(msg: string) {
  const para = document.createElement('p')

  para.appendChild(document.createTextNode($gettext(msg)))

  logContainer.value!.appendChild(para)

  logContainer.value?.scroll({ top: 100000, left: 0, behavior: 'smooth' })
}

async function issue_cert(config_name: string, server_name: string[], key_type: string) {
  return new Promise<CertificateResult>((resolve, reject) => {
    progressStatus.value = 'active'
    modalClosable.value = false
    modalVisible.value = true
    progressPercent.value = 0
    logContainer.value!.innerHTML = ''

    log($gettext('Getting the certificate, please wait...'))

    const ws = websocket(`/api/domain/${config_name}/cert`, false)

    ws.onopen = () => {
      ws.send(JSON.stringify({
        server_name,
        ...props.options,
        key_type,
      }))
    }

    ws.onmessage = async m => {
      const r = JSON.parse(m.data)

      log(T(r))

      switch (r.status) {
        case 'success':
          modalClosable.value = true
          issuingCert.value = false

          if (r.ssl_certificate !== undefined && r.ssl_certificate_key !== undefined) {
            progressStatus.value = 'success'
            progressPercent.value = 100
            resolve({
              ssl_certificate: r.ssl_certificate,
              ssl_certificate_key: r.ssl_certificate_key,
              key_type: r.key_type,
            })
          }
          break
        case 'error':
          modalClosable.value = true
          progressStatus.value = 'exception'
          reject($gettext('Fail to obtain certificate'))
          break
        default:
          // If it is a nginx ui log, increase the percent.
          if (r.message.includes('[Nginx UI]'))
            progressPercent.value += 8
          break
      }
    }
  })
}

defineExpose({
  issue_cert,
})
</script>

<template>
  <div>
    <AProgress
      :stroke-color="progressStrokeColor"
      :percent="progressPercent"
      :status="progressStatus"
    />

    <div
      ref="logContainer"
      class="issue-cert-log-container"
    />
  </div>
</template>

<style lang="less">
.dark {
  .issue-cert-log-container {
    background-color: rgba(0, 0, 0, 0.84);
  }
}

.issue-cert-log-container {
  height: 320px;
  overflow: scroll;
  background-color: #f3f3f3;
  border-radius: 4px;
  margin-top: 15px;
  padding: 10px;

  p {
    font-size: 12px;
    line-height: 1.3;
  }
}
</style>

<style scoped lang="less">

</style>
