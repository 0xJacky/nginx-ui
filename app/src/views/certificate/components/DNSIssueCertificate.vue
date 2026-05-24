<script setup lang="ts">
import type { Ref } from 'vue'
import type { AutoCertOptions } from '@/api/auto_cert'
import type { SelfSignedCertPayload } from '@/api/cert'
import cert from '@/api/cert'
import AutoCertForm from '@/components/AutoCertForm'
import StringListInput from '@/components/StringListInput'
import { PrivateKeyTypeEnum } from '@/constants'
import ObtainCertLive from '@/views/site/site_edit/components/Cert/ObtainCertLive.vue'
import SelfSignedCertFields from './SelfSignedCertFields.vue'

const emit = defineEmits<{
  issued: [void]
}>()

const { message } = App.useApp()

type CertType = 'wildcard' | 'custom' | 'self_signed'

const step = ref(0)
const visible = ref(false)
const data = ref({}) as Ref<AutoCertOptions>
const domain = ref('')
const certType = ref<CertType>('wildcard')
const customDomains = ref<string[]>([''])
const errored = ref(false)
const selfSignedLoading = ref(false)

function emptySelfSignedPayload(): SelfSignedCertPayload {
  return {
    name: '',
    domains: [''],
    ip_addresses: [''],
    key_type: PrivateKeyTypeEnum.P256,
    validity_days: 365,
    sync_node_ids: [],
  }
}

const selfSignedPayload = ref<SelfSignedCertPayload>(emptySelfSignedPayload())

function open() {
  visible.value = true
  step.value = 0
  data.value = {
    challenge_method: 'dns01',
    key_type: 'P256',
  } as AutoCertOptions
  domain.value = ''
  certType.value = 'wildcard'
  customDomains.value = ['']
  errored.value = false
  selfSignedPayload.value = emptySelfSignedPayload()
}

defineExpose({
  open,
})

const modalVisible = ref(false)
const modalClosable = ref(true)

const refObtainCertLive = useTemplateRef('refObtainCertLive')

const computedDomain = computed(() => {
  return `*.${domain.value}`
})

const computedDomains = computed(() => {
  if (certType.value === 'wildcard') {
    return [computedDomain.value, domain.value]
  }
  else {
    return customDomains.value.filter(d => d.trim())
  }
})

const computedMainDomain = computed(() => {
  if (certType.value === 'wildcard') {
    return computedDomain.value
  }
  else {
    return customDomains.value.find(d => d.trim()) || ''
  }
})

function issueCert() {
  if (!data.value.dns_credential_id) {
    message.error($gettext('Please select a DNS credential'))
    return
  }

  if (certType.value === 'custom') {
    const validDomains = customDomains.value.filter(d => d.trim())
    if (validDomains.length === 0) {
      message.error($gettext('Please enter at least one domain'))
      return
    }
  }

  errored.value = false
  step.value = 1
  modalVisible.value = true

  // ObtainCertLive is mounted in the same modal via force-render, so the
  // ref is guaranteed to be available by the time this function runs.
  refObtainCertLive.value!
    .issue_cert(computedMainDomain.value, computedDomains.value, data.value.key_type)
    .then(() => {
      message.success($gettext('Issued successfully'))
      emit('issued')
    })
    .catch(() => {
      errored.value = true
    })
}

async function submitSelfSigned() {
  const name = selfSignedPayload.value.name.trim()
  const domains = selfSignedPayload.value.domains.map(d => d.trim()).filter(Boolean)
  const ip_addresses = selfSignedPayload.value.ip_addresses.map(s => s.trim()).filter(Boolean)

  if (!name) {
    message.error($gettext('Please enter a name for the certificate'))
    return
  }
  if (domains.length === 0 && ip_addresses.length === 0) {
    message.error($gettext('Please enter at least one domain or IP address'))
    return
  }

  selfSignedLoading.value = true
  try {
    await cert.generate_self_signed({
      ...selfSignedPayload.value,
      name,
      domains,
      ip_addresses,
    })
    message.success($gettext('Self-signed certificate generated'))
    visible.value = false
    emit('issued')
  }
  // eslint-disable-next-line ts/no-explicit-any
  catch (e: any) {
    message.error(e.message ?? $gettext('Failed to generate self-signed certificate'))
  }
  finally {
    selfSignedLoading.value = false
  }
}
</script>

<template>
  <div>
    <AModal
      v-model:open="visible"
      :mask="false"
      :title="$gettext('Issue Certificate')"
      destroy-on-close
      :footer="null"
      :mask-closable="modalClosable"
      :closable="modalClosable"
      force-render
    >
      <template v-if="step === 0">
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Certificate Type')">
            <ASelect v-model:value="certType">
              <ASelectOption value="wildcard">
                {{ $gettext('Wildcard Certificate') }}
              </ASelectOption>
              <ASelectOption value="custom">
                {{ $gettext('Custom Domains Certificate') }}
              </ASelectOption>
              <ASelectOption value="self_signed">
                {{ $gettext('Self-signed Certificate') }}
              </ASelectOption>
            </ASelect>
          </AFormItem>

          <template v-if="certType === 'wildcard'">
            <AFormItem :label="$gettext('Domain')">
              <AInput
                v-model:value="domain"
                addon-before="*."
                :placeholder="$gettext('Enter your domain')"
              />
            </AFormItem>
          </template>

          <template v-else-if="certType === 'custom'">
            <AFormItem :label="$gettext('Custom Domains')">
              <StringListInput
                v-model="customDomains"
                :placeholder="$gettext('Enter domain name')"
                :add-button-text="$gettext('Add Domain')"
              />
              <AAlert
                :message="$gettext('All selected subdomains must belong to the same DNS Provider, otherwise the certificate application will fail.')"
                type="info"
                show-icon
                banner
                class="mt-3"
              />
            </AFormItem>
          </template>
        </AForm>

        <template v-if="certType !== 'self_signed'">
          <AutoCertForm
            v-model:options="data"
            style="max-width: 600px"
            hide-note
            force-dns-challenge
          />

          <div class="flex justify-end">
            <AButton
              type="primary"
              @click="issueCert"
            >
              {{ $gettext('Next') }}
            </AButton>
          </div>
        </template>

        <template v-else>
          <SelfSignedCertFields v-model="selfSignedPayload" />

          <div class="flex justify-end">
            <AButton
              type="primary"
              :loading="selfSignedLoading"
              @click="submitSelfSigned"
            >
              {{ $gettext('Generate') }}
            </AButton>
          </div>
        </template>
      </template>

      <ObtainCertLive
        v-show="step === 1"
        ref="refObtainCertLive"
        v-model:modal-closable="modalClosable"
        v-model:modal-visible="modalVisible"
        :options="data"
      />

      <div
        v-if="step === 1 && errored"
        class="flex justify-end mt-4"
      >
        <AButton
          type="primary"
          @click="issueCert"
        >
          {{ $gettext('Retry') }}
        </AButton>
      </div>
    </AModal>
  </div>
</template>

<style scoped lang="less">

</style>
