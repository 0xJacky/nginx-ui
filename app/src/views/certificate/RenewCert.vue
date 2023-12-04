<script setup lang="ts">
import { useGettext } from 'vue3-gettext'
import type { Ref } from 'vue'
import { message } from 'ant-design-vue'
import ObtainCertLive from '@/views/domain/cert/components/ObtainCertLive.vue'
import type { Cert } from '@/api/cert'

const emit = defineEmits<{
  renewed: [void]
}>()

const { $gettext } = useGettext()

const modalVisible = ref(false)
const modalClosable = ref(true)

const refObtainCertLive = ref()

const data = inject('data') as Ref<Cert>

const issue_cert = () => {
  modalVisible.value = true

  refObtainCertLive.value.issue_cert(data.value.name, data.value.domains, () => {
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
      @click="issue_cert"
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
