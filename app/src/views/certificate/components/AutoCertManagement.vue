<script setup lang="ts">
import type { Cert } from '@/api/cert'
import AutoCertForm from '@/components/AutoCertForm'

defineProps<{
  isManaged: boolean
}>()

const data = defineModel<Cert>('data', { required: true })
</script>

<template>
  <div class="auto-cert-management">
    <template v-if="isManaged">
      <AAlert
        v-if="!data.filename"
        class="mb-4"
        :title="$gettext('This Auto Cert item is invalid, please remove it.')"
        type="error"
        show-icon
      />
      <AAlert
        v-else-if="!data.domains"
        class="mb-4"
        :title="$gettext('Domains list is empty, try to reopen Auto Cert for %{config}', { config: data.filename })"
        type="error"
        show-icon
      />
    </template>

    <AutoCertForm
      v-model:options="data"
      key-type-read-only
      existing
      hide-note
    />
  </div>
</template>
