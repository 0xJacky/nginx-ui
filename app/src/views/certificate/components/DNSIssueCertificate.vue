<script setup lang="ts">
import type { Ref } from 'vue'
import type { AutoCertOptions } from '@/api/auto_cert'
import { message } from 'ant-design-vue'
import AutoCertForm from '@/components/AutoCertForm'
import ObtainCertLive from '@/views/site/site_edit/components/Cert/ObtainCertLive.vue'

const emit = defineEmits<{
  issued: [void]
}>()

const step = ref(0)
const visible = ref(false)
const data = ref({}) as Ref<AutoCertOptions>
const domain = ref('')
const certType = ref<'wildcard' | 'custom'>('wildcard')
const customDomains = ref<string[]>([''])

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

function addCustomDomain() {
  customDomains.value.push('')
}

function removeCustomDomain(index: number) {
  if (customDomains.value.length > 1) {
    customDomains.value.splice(index, 1)
  }
}

function issueCert() {
  if (certType.value === 'custom') {
    const validDomains = customDomains.value.filter(d => d.trim())
    if (validDomains.length === 0) {
      message.error($gettext('Please enter at least one domain'))
      return
    }
  }

  step.value++
  modalVisible.value = true

  refObtainCertLive.value?.issue_cert(computedMainDomain.value, computedDomains.value, data.value.key_type)
    .then(() => {
      message.success($gettext('Renew successfully'))
      emit('issued')
    })
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

          <template v-else>
            <AFormItem :label="$gettext('Custom Domains')">
              <div class="space-y-2">
                <div
                  v-for="(_, index) in customDomains"
                  :key="index"
                  class="flex items-center gap-2"
                >
                  <AInput
                    v-model:value="customDomains[index]"
                    :placeholder="$gettext('Enter domain name')"
                    class="flex-1"
                  />
                  <AButton
                    v-if="customDomains.length > 1"
                    type="link"
                    danger
                    @click="removeCustomDomain(index)"
                  >
                    {{ $gettext('Remove') }}
                  </AButton>
                </div>
                <AButton
                  block
                  @click="addCustomDomain"
                >
                  {{ $gettext('Add Domain') }}
                </AButton>
              </div>

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

        <AutoCertForm
          v-model:options="data"
          style="max-width: 600px"
          hide-note
          force-dns-challenge
        />

        <div
          v-if="step === 0"
          class="flex justify-end"
        >
          <AButton
            type="primary"
            @click="issueCert"
          >
            {{ $gettext('Next') }}
          </AButton>
        </div>
      </template>

      <ObtainCertLive
        v-show="step === 1"
        ref="refObtainCertLive"
        v-model:modal-closable="modalClosable"
        v-model:modal-visible="modalVisible"
        :options="data"
      />
    </AModal>
  </div>
</template>

<style scoped lang="less">

</style>
