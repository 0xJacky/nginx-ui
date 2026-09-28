<script setup lang="ts">
import { listOptions, PublicValue } from './options'
import { useAccessControlStore } from './store'

const props = withDefaults(defineProps<{
  serverIdx: number
  context?: 'http' | 'stream'
}>(), {
  context: 'http',
})

const store = useAccessControlStore()
const { lists, isApplying, isLoadingLists, stateError } = storeToRefs(store)

const state = computed(() => store.serverState(props.serverIdx))
const isManual = computed(() => state.value?.mode === 'manual')

const value = computed(() => state.value?.mode === 'list' ? state.value.slug : PublicValue)

const options = computed(() => [
  { label: $gettext('Public'), value: PublicValue },
  ...listOptions(lists.value, state.value?.slug),
])

const hint = computed(() => {
  if (state.value?.mode !== 'list')
    return $gettext('Everyone can reach this server.')
  const name = store.listName(state.value.slug)
  if (props.context === 'stream')
    return $gettext('Only addresses allowed by %{name} can connect.', { name })
  return $gettext('Only addresses allowed by %{name} can reach this server. Locations can inherit it, open up or use another list.', { name })
})

function onChange(selected: unknown) {
  if (typeof selected !== 'string')
    return
  store.apply([selected === PublicValue
    ? { server: props.serverIdx, mode: 'public' }
    : { server: props.serverIdx, mode: 'list', slug: selected }])
}
</script>

<template>
  <div class="mb-4">
    <AFlex align="center" justify="space-between" class="mb-2">
      <h3 class="mb-0!">
        {{ $gettext('Access Control') }}
      </h3>
      <RouterLink to="/access-lists">
        {{ $gettext('Manage access lists') }}
      </RouterLink>
    </AFlex>

    <AAlert
      v-if="stateError"
      class="mb-2"
      type="warning"
      show-icon
      :title="$gettext('The access state could not be read')"
      :description="stateError"
    />
    <AAlert
      v-else-if="isManual"
      class="mb-2"
      type="info"
      show-icon
      :title="$gettext('This server has allow or deny rules that were not written by Nginx UI')"
      :description="$gettext('Remove them from the directives to manage access with an access list.')"
    />

    <AFlex vertical gap="small">
      <ASelect
        class="w-full max-w-100"
        :value="isManual ? undefined : value"
        :placeholder="isManual ? $gettext('Custom rules') : undefined"
        :options="options"
        :disabled="!state || isManual"
        :loading="isApplying || isLoadingLists"
        data-testid="server-access-select"
        @change="onChange"
      />
      <ATypographyText v-if="!isManual" type="secondary" class="text-xs">
        {{ hint }}
      </ATypographyText>
    </AFlex>
  </div>
</template>
