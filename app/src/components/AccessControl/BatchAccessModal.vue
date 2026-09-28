<script setup lang="ts">
import type { AccessBatchResult, AccessList } from '@/api/access_list'
import accessList from '@/api/access_list'
import { listOptions, PublicValue } from './options'

const props = defineProps<{
  kind: 'site' | 'stream'
  names: string[]
}>()

const emit = defineEmits<{
  done: []
}>()

const open = defineModel<boolean>('open', { default: false })

const { message } = App.useApp()

const lists = ref<AccessList[]>([])
const selected = ref<string>()
const isApplying = ref(false)
const failures = ref<AccessBatchResult[]>([])

const options = computed(() => [
  { label: $gettext('Public'), value: PublicValue },
  ...listOptions(lists.value),
])

watch(open, async value => {
  if (!value)
    return
  selected.value = undefined
  failures.value = []
  lists.value = (await accessList.getAll()).data ?? []
})

async function apply() {
  if (!selected.value)
    return
  isApplying.value = true
  try {
    const { results } = await accessList.batchApply({
      kind: props.kind,
      names: props.names,
      mode: selected.value === PublicValue ? 'public' : 'list',
      slug: selected.value === PublicValue ? undefined : selected.value,
    })
    failures.value = results.filter(r => !r.success)
    const succeeded = results.length - failures.value.length
    if (succeeded > 0)
      message.success($ngettext('Access updated for %{count} item', 'Access updated for %{count} items', succeeded, { count: String(succeeded) }))
    emit('done')
    if (failures.value.length === 0)
      open.value = false
  }
  finally {
    isApplying.value = false
  }
}
</script>

<template>
  <AModal
    v-model:open="open"
    :title="kind === 'site' ? $gettext('Set access for selected sites') : $gettext('Set access for selected streams')"
    :ok-text="$gettext('Apply')"
    :cancel-text="$gettext('Cancel')"
    :ok-button-props="{ disabled: !selected }"
    :confirm-loading="isApplying"
    data-testid="batch-access-modal"
    @ok="apply"
  >
    <p>
      {{ $gettext('Every server block of the selected items uses the choice below. Locations with their own setting keep it. Each file is tested and Nginx reloaded.') }}
    </p>
    <ul class="max-h-40 overflow-auto pl-5">
      <li v-for="name in names" :key="name">
        {{ name }}
      </li>
    </ul>
    <ASelect
      v-model:value="selected"
      class="w-full"
      :options="options"
      :placeholder="$gettext('Select an access list')"
      data-testid="batch-access-select"
    />
    <AAlert
      v-if="failures.length > 0"
      class="mt-4"
      type="error"
      show-icon
      :title="$gettext('Some items were not changed')"
    >
      <template #description>
        <ul class="pl-5 mb-0">
          <li v-for="f in failures" :key="f.name">
            {{ f.name }}: {{ f.error }}
          </li>
        </ul>
      </template>
    </AAlert>
  </AModal>
</template>
