<script setup lang="ts">
import type { AutoCertOptions } from '@/api/auto_cert'
import type { Ref } from 'vue'
import AutoCertStepOne from '@/views/site/cert/components/AutoCertStepOne.vue'
import ObtainCertLive from '@/views/site/cert/components/ObtainCertLive.vue'
import { message } from 'ant-design-vue'

const emit = defineEmits<{
  issued: [void]
}>()

const step = ref(0)
const visible = ref(false)
const data = ref({}) as Ref<AutoCertOptions>
const issuing_cert = ref(false)
const domain = ref('')

provide('issuing_cert', issuing_cert)
function open() {
  visible.value = true
  step.value = 0
  data.value = {
    challenge_method: 'dns01',
    key_type: '2048',
  } as AutoCertOptions
  domain.value = ''
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

function issueCert() {
  step.value++
  modalVisible.value = true

  refObtainCertLive.value?.issue_cert(computedDomain.value, [computedDomain.value, domain.value], data.value.key_type)
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
      :title="$gettext('Issue Wildcard Certificate')"
      destroy-on-close
      :footer="null"
      :mask-closable="modalClosable"
      :closable="modalClosable"
      force-render
    >
      <template v-if="step === 0">
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Domain')">
            <AInput
              v-model:value="domain"
              addon-before="*."
            />
          </AFormItem>
        </AForm>

        <AutoCertStepOne
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
