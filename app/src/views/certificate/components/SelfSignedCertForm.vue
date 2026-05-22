<script setup lang="ts">
import type { Cert, SelfSignedCertPayload } from '@/api/cert'
import cert from '@/api/cert'
import { PrivateKeyTypeEnum } from '@/constants'
import SelfSignedCertFields from './SelfSignedCertFields.vue'

const props = defineProps<{
  defaultDomains?: string[]
}>()

const emit = defineEmits<{
  created: [cert: Cert]
}>()

const { message } = App.useApp()

const visible = ref(false)
const loading = ref(false)

function emptyForm(): SelfSignedCertPayload {
  return {
    name: '',
    domains: [...(props.defaultDomains ?? [])],
    ip_addresses: [],
    key_type: PrivateKeyTypeEnum.P256,
    validity_days: 365,
    sync_node_ids: [],
  }
}

const form = ref<SelfSignedCertPayload>(emptyForm())

function open() {
  form.value = emptyForm()
  visible.value = true
}

defineExpose({ open })

async function submit() {
  if (form.value.domains.length === 0 && form.value.ip_addresses.length === 0) {
    message.error($gettext('Please enter at least one domain or IP address'))
    return
  }

  loading.value = true
  try {
    const created = await cert.generate_self_signed(form.value)
    message.success($gettext('Self-signed certificate generated'))
    visible.value = false
    emit('created', created)
  }
  // eslint-disable-next-line ts/no-explicit-any
  catch (e: any) {
    message.error(e.message ?? $gettext('Failed to generate self-signed certificate'))
  }
  finally {
    loading.value = false
  }
}
</script>

<template>
  <AModal
    v-model:open="visible"
    :title="$gettext('Generate Self-signed Certificate')"
    :confirm-loading="loading"
    :ok-text="$gettext('Generate')"
    :width="600"
    destroy-on-close
    @ok="submit"
  >
    <SelfSignedCertFields v-model="form" />
  </AModal>
</template>
