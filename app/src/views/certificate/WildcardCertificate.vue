<script setup lang="ts">
import type { Ref } from 'vue'
import { message } from 'ant-design-vue'
import type { Cert } from '@/api/cert'
import ObtainCertLive from '@/views/domain/cert/components/ObtainCertLive.vue'
import DNSChallenge from '@/views/domain/cert/components/DNSChallenge.vue'

const emit = defineEmits<{
  issued: [void]
}>()

const step = ref(0)
const visible = ref(false)
const data = ref({}) as Ref<Cert>
const issuing_cert = ref(false)

provide('data', data)
provide('issuing_cert', issuing_cert)
function open() {
  visible.value = true
  step.value = 0
  data.value = {
    challenge_method: 'dns01',
    key_type: '2048',
  } as Cert
}

defineExpose({
  open,
})

const modalVisible = ref(false)
const modalClosable = ref(true)

const refObtainCertLive = ref()
const domain = ref('')

const computedDomain = computed(() => {
  return `*.${domain.value}`
})

const issueCert = () => {
  step.value++
  modalVisible.value = true

  refObtainCertLive.value.issue_cert(computedDomain.value,
    [computedDomain.value, domain.value], data.value.key_type)
    .then(() => {
      message.success($gettext('Renew successfully'))
      emit('issued')
    })
}

const keyType = shallowRef([
  {
    key: '2048',
    name: 'RSA2048',
  },
  {
    key: '3072',
    name: 'RSA3072',
  },
  {
    key: '4096',
    name: 'RSA4096',
  },
  {
    key: '8192',
    name: 'RAS8192',
  },
  {
    key: 'P256',
    name: 'EC256',
  },
  {
    key: 'P384',
    name: 'EC384',
  },
])
</script>

<template>
  <div>
    <AModal
      v-model:open="visible"
      :mask="false"
      :title="$gettext('Issue Wildcard Certificate')"
      destroy-on-close
      :footer="null"
      :mask-closable="modalClosable"
      :closable="modalClosable"
      force-render
    >
      <template v-if="step === 0">
        <DNSChallenge />

        <AForm layout="vertical">
          <AFormItem :label="$gettext('Domain')">
            <AInput
              v-model:value="domain"
              addon-before="*."
            />
          </AFormItem>

          <AFormItem :label="$gettext('Key Type')">
            <ASelect v-model:value="data.key_type">
              <ASelectOption
                v-for="t in keyType"
                :key="t.key"
                :value="t.key"
              >
                {{ t.name }}
              </ASelectOption>
            </ASelect>
          </AFormItem>
        </AForm>
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
      />
    </AModal>
  </div>
</template>

<style scoped lang="less">

</style>
