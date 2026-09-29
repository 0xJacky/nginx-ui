<script setup lang="ts">
import type { DnsCredential } from '@/api/dns_credential'
import dns_credential from '@/api/dns_credential'
import { useGlobalApp } from '@/composables/useGlobalApp'
import { getErrorMessage } from '@/lib/http'
import DnsCredentialForm from '@/views/dns/components/DnsCredentialForm.vue'
import { dnsCredentialEditorRequests, settleDnsCredentialEditor } from './openDnsCredentialEditor'

const { message } = useGlobalApp()

const current = computed(() => dnsCredentialEditorRequests.value[0])
const open = ref(false)
const saving = ref(false)
const draft = ref<DnsCredential>(emptyCredential())

function emptyCredential(): DnsCredential {
  return {
    name: '',
    code: '',
    provider: '',
    configuration: { credentials: {}, additional: {} },
  } as unknown as DnsCredential
}

watch(current, request => {
  if (!request)
    return
  draft.value = emptyCredential()
  open.value = true
}, { immediate: true })

function cancel() {
  if (saving.value || !current.value)
    return
  open.value = false
  settleDnsCredentialEditor(current.value.id)
}

async function save() {
  const request = current.value
  if (!request)
    return

  const name = draft.value.name?.trim()
  if (!name) {
    message.error($gettext('Name cannot be empty'))
    return
  }
  if (!draft.value.code) {
    message.error($gettext('Please select DNS provider'))
    return
  }

  saving.value = true
  try {
    const created = await dns_credential.createItem({ ...draft.value, name })
    message.success($gettext('Save successfully'))
    open.value = false
    settleDnsCredentialEditor(request.id, {
      id: created.id,
      name: created.name,
      code: created.code || created.provider_code || draft.value.code,
      provider: created.provider,
      provider_code: created.provider_code,
    })
  }
  catch (e) {
    message.error(getErrorMessage(e, $gettext('Server error')))
  }
  finally {
    saving.value = false
  }
}
</script>

<template>
  <AModal
    :open="open && !!current"
    :title="$gettext('New DNS Credential')"
    :closable="!saving"
    :mask-closable="!saving"
    :z-index="1100"
    destroy-on-hidden
    width="680"
    @cancel="cancel"
  >
    <DnsCredentialForm v-if="current" :key="current.id" v-model:data="draft" />
    <template #footer>
      <AButton :disabled="saving" @click="cancel">
        {{ $gettext('Cancel') }}
      </AButton>
      <AButton type="primary" :loading="saving" @click="save">
        {{ $gettext('Create and select') }}
      </AButton>
    </template>
  </AModal>
</template>
