<script setup lang="ts">
import type { AccessChange } from '@/api/access_list'
import { listOptions, PublicValue, serverAccessLabel, sharedServerAccessValue } from './options'
import { useAccessControlStore } from './store'

const store = useAccessControlStore()
const { servers, lists, isApplying } = storeToRefs(store)

function customizedLocations(index: number) {
  return servers.value[index]?.locations?.filter(l => l.mode !== 'inherit' && l.mode !== 'acme').length ?? 0
}

const hasManual = computed(() => servers.value.some(s => s.mode === 'manual'))

// Shows the access every server shares; empty (with the placeholder) when
// they differ.
const sharedValue = computed(() => sharedServerAccessValue(servers.value))

const options = computed(() => [
  { label: $gettext('Public'), value: PublicValue },
  ...listOptions(lists.value, sharedValue.value === PublicValue ? undefined : sharedValue.value),
])

function applyToAll(selected: unknown) {
  if (typeof selected !== 'string')
    return
  const changes: AccessChange[] = servers.value.map(s => selected === PublicValue
    ? { server: s.index, mode: 'public' }
    : { server: s.index, mode: 'list', slug: selected })
  store.apply(changes)
}
</script>

<template>
  <div v-if="servers.length > 0">
    <div class="flex flex-col gap-2 mb-3">
      <AFlex
        v-for="server in servers"
        :key="server.index"
        justify="space-between"
        align="center"
        gap="small"
      >
        <div class="min-w-0">
          <div class="truncate">
            {{ server.server_name || $gettext('Server %{index}', { index: String(server.index + 1) }) }}
          </div>
          <ATypographyText
            v-if="customizedLocations(server.index) > 0"
            type="secondary"
            class="text-xs"
          >
            {{ $ngettext('%{count} location set separately', '%{count} locations set separately', customizedLocations(server.index), { count: String(customizedLocations(server.index)) }) }}
          </ATypographyText>
        </div>
        <ATag :color="server.mode === 'list' ? 'green' : 'default'" :bordered="false" class="mr-0!">
          {{ serverAccessLabel(server, store.listName) }}
        </ATag>
      </AFlex>
    </div>
    <!-- Also shown for a single server: in advanced mode this is the only
         control, since the server cards belong to the basic mode editor. -->
    <ASelect
      :value="sharedValue"
      class="w-full"
      :placeholder="servers.length > 1 ? $gettext('Use for all servers') : $gettext('Change access')"
      :options="options"
      :disabled="hasManual"
      :loading="isApplying"
      @change="applyToAll"
    />
  </div>
</template>
