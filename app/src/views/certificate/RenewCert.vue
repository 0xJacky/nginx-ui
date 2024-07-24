<script setup lang="ts">
import { message } from 'ant-design-vue'
import ObtainCertLive from '@/views/domain/cert/components/ObtainCertLive.vue'
import type { AutoCertOptions } from '@/api/auto_cert'

const props = defineProps<{
  options: AutoCertOptions
}>()

const emit = defineEmits<{
  renewed: [void]
}>()

const modalVisible = ref(false)
const modalClosable = ref(true)

const refObtainCertLive = ref()

const issueCert = () => {
  modalVisible.value = true

  const { name, domains, key_type } = props.options

  refObtainCertLive.value.issue_cert(name, domains, key_type).then(() => {
    message.success($gettext('Renew successfully'))
    emit('renewed')
  })
}

const issuing_cert = ref(false)

provide('issuing_cert', issuing_cert)
</script>

<template>
  <div>
    <AButton
      type="primary"
      ghost
      class="mb-6"
      @click="issueCert"
    >
      {{ $gettext('Renew Certificate') }}
    </AButton>
    <AModal
      v-model:open="modalVisible"
      :title="$gettext('Renew Certificate')"
      :mask-closable="modalClosable"
      :footer="null"
      :closable="modalClosable"
      :width="600"
      force-render
    >
      <ObtainCertLive
        ref="refObtainCertLive"
        v-model:modal-closable="modalClosable"
        v-model:modal-visible="modalVisible"
        :options
      />
    </AModal>
  </div>
</template>

<style lang="less" scoped>
.control-btn {
  display: flex;
  justify-content: flex-end;
}
</style>
