<script setup lang="ts">
import type { ProxyLocationChange } from './proxyPassUpstream'
import { useManagedUpstreams } from '@/composables/useManagedUpstreams'
import {
  applyUpstreamToLocations,
  defaultProxyLocationKeys,
  findUnreferencedUpstreams,
  listProxyLocations,
} from './proxyPassUpstream'
import { useNgxConfigStore } from './store'

const open = defineModel<boolean>('open', { default: false })

const { ngxConfig } = storeToRefs(useNgxConfigStore())
const { upstreams, options: upstreamOptions, isLoading, reload } = useManagedUpstreams()

const selectedGroup = ref<string>()
const selectedKeys = ref<string[]>([])
// Set once the locations were rewritten; the dialog then lists the changes.
const changes = ref<ProxyLocationChange[] | null>(null)
// Upstream blocks of this site that the change left without any user.
const unusedUpstreams = ref<string[]>([])

const locations = computed(() => listProxyLocations(ngxConfig.value))

watch(open, isOpen => {
  if (!isOpen)
    return
  // Pick up groups created on the Upstream Groups page since the editor opened.
  reload()
  changes.value = null
  unusedUpstreams.value = []
  selectedGroup.value = undefined
  selectedKeys.value = defaultProxyLocationKeys(locations.value)
})

const selectedGroupDetail = computed(() => upstreams.value.find(item => item.name === selectedGroup.value))

const canApply = computed(() => !!selectedGroup.value && selectedKeys.value.length > 0)

function serverLabel(serverName: string, serverIdx: number) {
  return serverName || $gettext('Server %{index}', { index: String(serverIdx + 1) })
}

function apply() {
  if (!canApply.value)
    return
  const siteUpstreams = (ngxConfig.value.upstreams ?? []).map(item => item.name).filter(Boolean)
  const unusedBefore = findUnreferencedUpstreams(ngxConfig.value, siteUpstreams)
  const usedBefore = siteUpstreams.filter(name => !unusedBefore.includes(name))
  changes.value = applyUpstreamToLocations(ngxConfig.value, selectedGroup.value!, selectedKeys.value)
  unusedUpstreams.value = findUnreferencedUpstreams(ngxConfig.value, usedBefore)
}

function close() {
  open.value = false
}
</script>

<template>
  <AModal
    v-model:open="open"
    :title="$gettext('Use an Existing Upstream Group')"
    centered
    :width="560"
  >
    <!-- Result: which locations now proxy to the group -->
    <div
      v-if="changes"
      data-testid="use-upstream-group-result"
    >
      <AAlert
        type="success"
        show-icon
        class="mb-4"
        :title="changes.length > 0
          ? $gettext('%{count} location(s) now proxy to %{name}', { count: String(changes.length), name: selectedGroup! })
          : $gettext('The selected locations already proxy to %{name}', { name: selectedGroup! })"
        :description="$gettext('Save the site to apply the change. Nginx is tested and reloaded on save.')"
      />
      <div
        v-for="change in changes"
        :key="change.key"
        class="mb-2 break-all"
      >
        <div class="font-medium">
          {{ serverLabel(change.serverName, change.serverIdx) }} · location {{ change.path }}
        </div>
        <div class="font-mono text-xs text-gray-500 dark:text-gray-400">
          {{ change.target ?? $gettext('no proxy_pass') }} → {{ change.after }}
        </div>
      </div>
      <AAlert
        v-if="unusedUpstreams.length > 0"
        type="info"
        show-icon
        class="mt-4"
        :title="$gettext('No location uses %{names} anymore. You can delete it in the Upstream section, or keep it.', { names: unusedUpstreams.join(', ') })"
        data-testid="use-upstream-group-unused"
      />
    </div>

    <!-- No managed groups yet -->
    <AEmpty
      v-else-if="!isLoading && upstreamOptions.length === 0"
      :description="$gettext('There are no upstream groups yet.')"
      data-testid="use-upstream-group-empty"
    >
      <RouterLink
        to="/upstream/list"
        @click="close"
      >
        {{ $gettext('Create one on the Upstream Groups page') }}
      </RouterLink>
    </AEmpty>

    <AForm
      v-else
      layout="vertical"
    >
      <p class="mt-0 text-gray-500 dark:text-gray-400">
        {{ $gettext('An upstream group is defined once on the Upstream Groups page and shared by every site that proxies to it.') }}
      </p>
      <AFormItem :label="$gettext('Upstream Group')">
        <ASelect
          v-model:value="selectedGroup"
          class="w-full"
          show-search
          :loading="isLoading"
          :options="upstreamOptions"
          :placeholder="$gettext('Select an upstream')"
          data-testid="use-upstream-group-select"
        />
        <div
          v-if="selectedGroupDetail"
          class="mt-1 font-mono text-xs text-gray-500 dark:text-gray-400 break-all"
        >
          {{ selectedGroupDetail.servers.map(server => server.address).join(', ') }}
        </div>
      </AFormItem>
      <AFormItem
        :label="$gettext('Locations')"
        :extra="$gettext('The selected locations proxy to the group. The scheme and URI of an existing proxy_pass are kept; locations without one get proxy_pass added.')"
      >
        <span
          v-if="locations.length === 0"
          class="text-gray-500 dark:text-gray-400"
        >
          {{ $gettext('This site has no locations yet.') }}
        </span>
        <ACheckboxGroup
          v-else
          v-model:value="selectedKeys"
          class="w-full"
          data-testid="use-upstream-group-locations"
        >
          <div class="flex flex-col gap-2 w-full">
            <ACheckbox
              v-for="location in locations"
              :key="location.key"
              :value="location.key"
            >
              <span class="break-all">
                {{ serverLabel(location.serverName, location.serverIdx) }} · location {{ location.path }}
              </span>
              <span class="block font-mono text-xs text-gray-500 dark:text-gray-400 break-all">
                {{ location.target ?? $gettext('no proxy_pass') }}
              </span>
            </ACheckbox>
          </div>
        </ACheckboxGroup>
      </AFormItem>
    </AForm>

    <template #footer>
      <AButton
        v-if="changes"
        type="primary"
        @click="close"
      >
        {{ $gettext('Done') }}
      </AButton>
      <template v-else>
        <AButton @click="close">
          {{ $gettext('Cancel') }}
        </AButton>
        <AButton
          type="primary"
          :disabled="!canApply"
          data-testid="use-upstream-group-apply"
          @click="apply"
        >
          {{ $gettext('Apply') }}
        </AButton>
      </template>
    </template>
  </AModal>
</template>
