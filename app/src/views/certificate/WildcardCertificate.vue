<script setup lang="ts">
import type { Ref } from 'vue'
import { message } from 'ant-design-vue'
import { useGettext } from 'vue3-gettext'
import type { Cert } from '@/api/cert'
import ObtainCertLive from '@/views/domain/cert/components/ObtainCertLive.vue'
import DNSChallenge from '@/views/domain/cert/components/DNSChallenge.vue'

const emit = defineEmits<{
  issued: [void]
}>()

const { $gettext } = useGettext()
const step = ref(0)
const visible = ref(false)
const data = ref({}) as Ref<Cert>
const issuing_cert = ref(false)

provide('data', data)
provide('issuing_cert', issuing_cert)
function open() {
  visible.value = true
  data.value = {
    challenge_method: 'dns01',
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

  refObtainCertLive.value.issue_cert(computedDomain.value, [computedDomain.value], () => {
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
        </AForm>
        <div
          v-if="step === 0"
          class="flex justify-end"
        >
          <AButton
            type="primary"
            @click="issueCert"
          >
            {{ $gettext('Issue') }}
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