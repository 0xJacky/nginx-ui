<script setup lang="ts">
import type { Ref } from 'vue'
import type { Cert, DiscoveredCertificatePair, SelfSignedCertPayload } from '@/api/cert'
import cert, { toSelfSignedPayload } from '@/api/cert'
import { AutoCertState, normalizePrivateKeyType } from '@/constants'

import AutoCertManagement from './components/AutoCertManagement.vue'
import CertificateActions from './components/CertificateActions.vue'
import CertificateBasicInfo from './components/CertificateBasicInfo.vue'
import CertificateContentEditor from './components/CertificateContentEditor.vue'
import CertificateDownload from './components/CertificateDownload.vue'
import SelfSignedCertManagement from './components/SelfSignedCertManagement.vue'
import { useCertStore } from './store'

const { message } = App.useApp()

const route = useRoute()
const certStore = useCertStore()
const router = useRouter()
const errors = ref({}) as Ref<Record<string, string>>
const importMode = ref<'paths' | 'directory'>('paths')
const importDir = ref('')
const discoveredPair = ref<DiscoveredCertificatePair>()
const discovering = ref(false)

const id = computed(() => {
  return Number.parseInt(route.params.id as string)
})

const { data } = storeToRefs(certStore)

const isManaged = computed(() => {
  return data.value.auto_cert === AutoCertState.Enable || data.value.auto_cert === AutoCertState.Sync
})

const isSelfSigned = computed(() => {
  return data.value.auto_cert === AutoCertState.SelfSigned
})

const isNewCert = computed(() => !(id.value > 0))

const isDirectoryImportMode = computed(() => {
  return isNewCert.value && !isSelfSigned.value && importMode.value === 'directory'
})

const selfSignedPayload = ref<SelfSignedCertPayload>()

watch(data, value => {
  if (value.auto_cert === AutoCertState.SelfSigned)
    selfSignedPayload.value = toSelfSignedPayload(value)
}, { immediate: true })

function init() {
  if (id.value > 0) {
    cert.getItem(id.value).then(r => {
      // Backend stores key_type in its canonical form (EC256, RSA2048…); the
      // ACME form's ASelect options use the legacy keys (P256, 2048…). Normalize
      // on load so the dropdown highlights the right option when editing.
      data.value = { ...r, key_type: normalizePrivateKeyType(r.key_type) }
    })
  }
  else {
    data.value = {} as Cert
    importMode.value = 'paths'
    importDir.value = ''
    discoveredPair.value = undefined
  }
}

onMounted(() => {
  init()
})

async function discoverDirectoryImport() {
  const dir = importDir.value.trim()
  if (!dir) {
    message.error($gettext('Please enter a certificate directory'))
    return
  }

  discovering.value = true
  try {
    const result = await cert.discover_existing({
      name: data.value.name,
      dir,
    })
    discoveredPair.value = result
    data.value = {
      ...data.value,
      name: data.value.name || result.name || '',
      ssl_certificate_path: result.ssl_certificate_path,
      ssl_certificate_key_path: result.ssl_certificate_key_path,
      key_type: normalizePrivateKeyType(result.key_type),
      certificate_info: result.certificate_info ?? data.value.certificate_info,
    }
    message.success($gettext('Certificate pair detected'))
  }
  // eslint-disable-next-line ts/no-explicit-any
  catch (e: any) {
    discoveredPair.value = undefined
    message.error(e.message ?? $gettext('Failed to detect certificate files'))
  }
  finally {
    discovering.value = false
  }
}

async function save() {
  try {
    let savedId = data.value.id
    if (isDirectoryImportMode.value) {
      const name = data.value.name?.trim()
      const dir = importDir.value.trim()

      if (!name) {
        message.error($gettext('Please enter a name for the certificate'))
        return
      }
      if (!dir) {
        message.error($gettext('Please enter a certificate directory'))
        return
      }

      const result = await cert.import_existing({
        name,
        dir,
      })
      savedId = result.id
      data.value = { ...result, key_type: normalizePrivateKeyType(result.key_type) }
    }
    else if (isSelfSigned.value && selfSignedPayload.value && data.value.id) {
      const payload = selfSignedPayload.value
      const name = payload.name.trim()
      const domains = payload.domains.map(d => d.trim()).filter(Boolean)
      const ip_addresses = payload.ip_addresses.map(s => s.trim()).filter(Boolean)

      if (!name) {
        message.error($gettext('Please enter a name for the certificate'))
        return
      }
      if (domains.length === 0 && ip_addresses.length === 0) {
        message.error($gettext('Please enter at least one domain or IP address'))
        return
      }

      const currentId = data.value.id
      const result = await cert.modify_self_signed(currentId, {
        ...payload,
        name,
        domains,
        ip_addresses,
      })
      savedId = result.id || currentId
      data.value = { ...result, id: savedId }
    }
    else {
      await certStore.save()
      savedId = data.value.id
    }
    if (!savedId) {
      message.error($gettext('Saved certificate response is missing an ID'))
      return
    }
    message.success($gettext('Save successfully'))
    errors.value = {}
    await router.push(`/certificates/${savedId}`)
  }
  // eslint-disable-next-line ts/no-explicit-any
  catch (e: any) {
    errors.value = e.errors ?? {}
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
        <!-- Self-signed Certificate Management -->
        <SelfSignedCertManagement
          v-if="isSelfSigned && selfSignedPayload"
          v-model:value="selfSignedPayload"
          :certificate-info="data.certificate_info"
        />

        <!-- Auto Certificate Management -->
        <AutoCertManagement
          v-else
          v-model:data="data"
          :is-managed="isManaged"
          @renewed="init"
        />

        <AForm layout="vertical">
          <AFormItem
            v-if="isNewCert && !isSelfSigned"
            :label="$gettext('Import Mode')"
          >
            <ARadioGroup
              v-model:value="importMode"
              option-type="button"
              button-style="solid"
            >
              <ARadioButton value="paths">
                {{ $gettext('Certificate files') }}
              </ARadioButton>
              <ARadioButton value="directory">
                {{ $gettext('Import from directory') }}
              </ARadioButton>
            </ARadioGroup>
          </AFormItem>

          <template v-if="isDirectoryImportMode">
            <AFormItem
              :label="$gettext('Name')"
              :validate-status="errors?.name ? 'error' : ''"
              :help="errors?.name?.includes('required')
                ? $gettext('This field is required')
                : ''"
            >
              <AInput v-model:value="data.name" />
            </AFormItem>

            <AFormItem :label="$gettext('Certificate Directory')">
              <AInputSearch
                v-model:value="importDir"
                :enter-button="$gettext('Detect')"
                :loading="discovering"
                @search="discoverDirectoryImport"
              />
            </AFormItem>

            <AAlert
              v-if="discoveredPair"
              type="success"
              show-icon
              class="detected-cert"
            >
              <template #message>
                {{ $gettext('Certificate pair detected') }}
              </template>
              <template #description>
                <div class="detected-paths">
                  <div>
                    <strong>{{ $gettext('SSL Certificate Path') }}</strong>
                    <span>{{ discoveredPair.ssl_certificate_path }}</span>
                  </div>
                  <div>
                    <strong>{{ $gettext('SSL Certificate Key Path') }}</strong>
                    <span>{{ discoveredPair.ssl_certificate_key_path }}</span>
                  </div>
                </div>
              </template>
            </AAlert>
          </template>

          <!-- Certificate Basic Information -->
          <CertificateBasicInfo
            v-if="!isSelfSigned && !isDirectoryImportMode"
            v-model:data="data"
            :errors="errors"
            :is-managed="isManaged"
          />

          <!-- Download Certificate Files -->
          <CertificateDownload
            v-if="!isDirectoryImportMode"
            :data="data"
          />

          <!-- Certificate Content Editor -->
          <CertificateContentEditor
            v-if="!isDirectoryImportMode"
            v-model:data="data"
            :errors="errors"
            :readonly="isManaged || isSelfSigned"
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

.detected-cert {
  max-width: 600px;
  margin-bottom: 16px;
}

.detected-paths {
  display: grid;
  gap: 8px;

  div {
    display: grid;
    gap: 2px;
  }

  span {
    word-break: break-all;
  }
}
</style>
