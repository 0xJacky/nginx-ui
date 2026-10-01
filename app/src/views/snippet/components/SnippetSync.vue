<script setup lang="ts">
import snippet from '@/api/snippet'
import NodeSelector from '@/components/NodeSelector'
import { useClusterSync } from '@/composables/useClusterSync'

const emit = defineEmits<{
  saved: [nodeIds: number[]]
}>()

const open = defineModel<boolean>('open', { default: false })

const { message } = App.useApp()
const { report } = useClusterSync()

const syncNodeIds = ref<number[]>([])
const syncOverwrite = ref(false)
const isLoading = ref(false)
const isSaving = ref(false)

watch(open, async value => {
  if (!value)
    return
  isLoading.value = true
  try {
    const current = await snippet.getSync()
    syncNodeIds.value = current.sync_node_ids ?? []
    syncOverwrite.value = current.sync_overwrite
  }
  finally {
    isLoading.value = false
  }
})

async function save() {
  isSaving.value = true
  try {
    const summary = await snippet.saveSync({ sync_node_ids: syncNodeIds.value, sync_overwrite: syncOverwrite.value })
    if (syncNodeIds.value.length > 0)
      report(summary)
    else
      message.success($gettext('Snippets are no longer synchronized'))
    emit('saved', syncNodeIds.value)
    open.value = false
  }
  catch {
    // The request layer already shows the error.
  }
  finally {
    isSaving.value = false
  }
}
</script>

<template>
  <AModal
    v-model:open="open"
    :title="$gettext('Synchronize Snippets')"
    :confirm-loading="isSaving"
    :ok-text="$gettext('Save')"
    destroy-on-hidden
    @ok="save"
  >
    <ASpin :spinning="isLoading">
      <p class="mt-0 text-gray-500 dark:text-gray-400">
        {{ $gettext('Every snippet is copied to the selected nodes now, and later changes follow automatically. Clear the selection to stop synchronizing.') }}
      </p>
      <NodeSelector
        v-model:target="syncNodeIds"
        hidden-local
      />
      <ACheckbox
        v-model:checked="syncOverwrite"
        class="mt-3"
      >
        {{ $gettext('Replace snippets that already exist on the nodes') }}
      </ACheckbox>
    </ASpin>
  </AModal>
</template>
