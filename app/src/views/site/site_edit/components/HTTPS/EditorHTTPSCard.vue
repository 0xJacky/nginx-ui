<script setup lang="ts">
import type { HTTPSResult } from '@/api/https'
import { useSiteEditorStore } from '../SiteEditor/store'
import HTTPSCard from './HTTPSCard.vue'
import { hasPendingTLSServer, hasUnsavedChanges, pendingTLSServersDiffer } from './siteHTTPSState'

// HTTPSCard bound to the site editor store: the backend rewrites the saved
// site file, so the card stays disabled while the editor has unsaved changes.

withDefaults(defineProps<{
  domains: string[]
  compact?: boolean
  // Lets the user fold the card, see HTTPSCard.
  collapsible?: boolean
  // Offers the card's "Existing certificate" method.
  existingCertificate?: boolean
}>(), {
  compact: false,
  collapsible: false,
  existingCertificate: true,
})

const emit = defineEmits<{
  success: [result: HTTPSResult]
}>()

defineSlots<{
  actions?: () => unknown
}>()

const { message } = useGlobalApp()
const editorStore = useSiteEditorStore()
const { name, data, ngxConfig, saving } = storeToRefs(editorStore)

const card = useTemplateRef('card')

// The config as last loaded from or saved to the backend.
const savedConfig = computed(() => data.value?.tokenized)

const isDirty = computed(() => hasUnsavedChanges(ngxConfig.value, savedConfig.value))
const unsavedPendingTLS = computed(() => pendingTLSServersDiffer(ngxConfig.value, savedConfig.value))
const hasPendingTLSInFile = computed(() => hasPendingTLSServer(savedConfig.value))

const running = computed(() => card.value?.running ?? false)

function onSuccess(result: HTTPSResult) {
  message.success($gettext('HTTPS is enabled'))
  emit('success', result)
}

async function saveFirst() {
  try {
    // An enabled site cannot hold a TLS server without certificate (nginx -t
    // rejects it), so it is left out; HTTPS setup generates it again.
    await editorStore.save({ omitIncompleteTLSServers: true })
    message.success($gettext('Saved successfully'))
  }
  catch {
    // The store already surfaces the error.
  }
}

defineExpose({
  running,
  reset: () => card.value?.reset(),
})
</script>

<template>
  <AFlex vertical gap="small">
    <AAlert
      v-if="isDirty && !running"
      type="warning"
      show-icon
      :title="$gettext('Save your changes before enabling HTTPS')"
    >
      <template #description>
        <AFlex wrap gap="small" align="center">
          <span>{{ $gettext('HTTPS setup rewrites the saved site configuration, so unsaved edits would be lost.') }}</span>
          <AButton
            type="primary"
            size="small"
            :loading="saving"
            @click="saveFirst"
          >
            {{ $gettext('Save') }}
          </AButton>
        </AFlex>
      </template>
    </AAlert>
    <AAlert
      v-else-if="unsavedPendingTLS && !running"
      type="info"
      show-icon
      :title="$gettext('HTTPS setup builds the HTTPS server block from the saved configuration. Unsaved edits to that block are not used.')"
    />

    <HTTPSCard
      ref="card"
      :config-name="name"
      :domains
      :compact
      :collapsible
      :has-pending-t-l-s-server="hasPendingTLSInFile"
      :disabled="isDirty"
      :skippable="false"
      :existing-certificate
      @success="onSuccess"
    >
      <template #actions>
        <slot name="actions" />
      </template>
    </HTTPSCard>
  </AFlex>
</template>
