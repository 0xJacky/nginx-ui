<script setup lang="ts">
import type { Ref } from 'vue'
import type { Cert } from '@/api/cert'
import { message } from 'ant-design-vue'
import cert from '@/api/cert'
import { AutoCertState } from '@/constants'

import AutoCertManagement from './components/AutoCertManagement.vue'
import CertificateActions from './components/CertificateActions.vue'
import CertificateBasicInfo from './components/CertificateBasicInfo.vue'
import CertificateContentEditor from './components/CertificateContentEditor.vue'
import CertificateDownload from './components/CertificateDownload.vue'
import { useCertStore } from './store'

const route = useRoute()
const certStore = useCertStore()
const router = useRouter()
const errors = ref({}) as Ref<Record<string, string>>

const id = computed(() => {
  return Number.parseInt(route.params.id as string)
})

const { data } = storeToRefs(certStore)

const isManaged = computed(() => {
  return data.value.auto_cert === AutoCertState.Enable || data.value.auto_cert === AutoCertState.Sync
})

function init() {
  if (id.value > 0) {
    cert.getItem(id.value).then(r => {
      data.value = r
    })
  }
  else {
    data.value = {} as Cert
  }
}

onMounted(() => {
  init()
})

async function save() {
  try {
    await certStore.save()
    errors.value = {}
    await router.push(`/certificates/${certStore.data.id}`)
  }
  // eslint-disable-next-line ts/no-explicit-any
  catch (e: any) {
    errors.value = e.errors
    message.error(e.message ?? $gettext('Server error'))
  }
}

function handleBack() {
  router.push('/certificates/list')
}

const log = computed(() => {
  if (!data.value.log)
    return ''

  return data.value.log.split('\n').map(line => {
    try {
      return T(JSON.parse(line))
    }
    catch {
      // fallback to legacy log format
      const matches = line.match(/\[Nginx UI\] (.*)/)
      if (matches?.[1])
        return line.replaceAll(matches[1], $gettext(matches[1]))
      return line
    }
  }).join('\n')
})
</script>

<template>
  <ACard :title="id > 0 ? $gettext('Modify Certificate') : $gettext('Import Certificate')">
    <ARow :gutter="[16, 16]">
      <ACol
        :sm="24"
        :lg="12"
      >
        <!-- Auto Certificate Management -->
        <AutoCertManagement
          v-model:data="data"
          :is-managed="isManaged"
          @renewed="init"
        />

        <AForm layout="vertical">
          <!-- Certificate Basic Information -->
          <CertificateBasicInfo
            v-model:data="data"
            :errors="errors"
            :is-managed="isManaged"
          />

          <!-- Download Certificate Files -->
          <CertificateDownload :data="data" />

          <!-- Certificate Content Editor -->
          <CertificateContentEditor
            v-model:data="data"
            :errors="errors"
            :readonly="isManaged"
            class="max-w-600px"
          />
        </AForm>
      </ACol>

      <!-- Log Column for Auto Cert -->
      <ACol
        v-if="data.auto_cert === AutoCertState.Enable"
        :sm="24"
        :lg="12"
      >
        <ACard size="small" :title="$gettext('Log')">
          <pre
            v-dompurify-html="log"
            class="log-container"
          />
        </ACard>
      </ACol>
    </ARow>

    <!-- Certificate Actions -->
    <CertificateActions
      @save="save"
      @back="handleBack"
    />
  </ACard>
</template>

<style scoped lang="less">
.log-container {
  overflow: scroll;
  padding: 5px;
  margin-bottom: 0;

  font-size: 12px;
  line-height: 2;
}

.code-editor-container {
  position: relative;

  .drag-overlay {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: rgba(24, 144, 255, 0.1);
    border: 2px dashed #1890ff;
    border-radius: 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 10;

    .drag-content {
      text-align: center;
      color: #1890ff;

      .drag-icon {
        font-size: 48px;
        margin-bottom: 16px;
        display: block;
      }

      p {
        font-size: 16px;
        margin: 0;
        font-weight: 500;
      }
    }
  }
}
</style>
