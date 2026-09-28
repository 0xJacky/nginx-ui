<script setup lang="ts">
import type { TableColumnsType } from 'antdv-next'
import type { RouteLocationRaw } from 'vue-router'
import type { ServerHealthState } from './serverHealth'
import type {
  ExternalUpstream,
  ManagedUpstreamDetail,
  ManagedUpstreamServer,
  UpstreamGroupState,
  UpstreamServerState,
  UpstreamSource,
} from '@/api/upstream'
import { PlusOutlined, ReloadOutlined, SearchOutlined } from '@antdv-next/icons'
import { breakpointsAntDesign, createReusableTemplate, useBreakpoints, watchDebounced } from '@vueuse/core'
import { useRouteQuery } from '@vueuse/router'
import upstream from '@/api/upstream'
import ConvertUpstreamModal from '@/components/NgxConfigEditor/ConvertUpstreamModal.vue'
import { useProxyAvailability } from '@/composables/useProxyAvailability'
import UpstreamEditor from './components/UpstreamEditor.vue'
import UpstreamServerLine from './components/UpstreamServerLine.vue'
import { disablesLastPrimary, filterUpstreams, normalizeKeyword } from './serverFilter'
import { serverHealth } from './serverHealth'

const { message, modal } = App.useApp()

const router = useRouter()

const upstreams = ref<ManagedUpstreamDetail[]>([])
const externalUpstreams = ref<ExternalUpstream[]>([])
const managedDir = ref('')
const isLoading = ref(false)

const isEditorOpen = ref(false)
const editingName = ref('')

// Live health of every server, shared with the other upstream-aware pages.
const proxyAvailabilityStore = useProxyAvailability()

const methodLabels: Record<string, () => string> = {
  '': () => $gettext('Round Robin'),
  'least_conn': () => $gettext('Least Connections'),
  'ip_hash': () => $gettext('IP Hash'),
  'hash': () => $gettext('Hash'),
  'random': () => $gettext('Random'),
}

// On a phone every group collapses into one column: name, actions or source,
// then the server switches, so nothing has to be scrolled sideways to reach a
// switch. Wider screens get the full table.
const isNarrow = useBreakpoints(breakpointsAntDesign).smaller('md')

const compactColumn = { title: () => $gettext('Upstream'), key: 'compact' }

const columns = computed<TableColumnsType<ManagedUpstreamDetail>>(() => isNarrow.value
  ? [compactColumn]
  : [
      { title: () => $gettext('Name'), key: 'name', width: 160 },
      { title: () => $gettext('Load Balancing'), key: 'method', width: 200 },
      { title: () => $gettext('Servers'), key: 'servers' },
      { title: () => $gettext('Referenced By'), key: 'references', width: 220, responsive: ['lg'] },
      { title: () => $gettext('Actions'), key: 'actions', width: 130, fixed: 'right' },
    ])

const externalColumns = computed<TableColumnsType<ExternalUpstream>>(() => isNarrow.value
  ? [compactColumn]
  : [
      { title: () => $gettext('Name'), key: 'name', width: 160 },
      { title: () => $gettext('Servers'), key: 'servers' },
      { title: () => $gettext('Defined In'), key: 'source', width: 260 },
    ])

// Cell fragments shared by the full and the compact layout.
const [DefineManagedServers, ManagedServers] = createReusableTemplate<{ record: ManagedUpstreamDetail }>()
const [DefineManagedActions, ManagedActions] = createReusableTemplate<{ record: ManagedUpstreamDetail }>()
const [DefineExternalServers, ExternalServers] = createReusableTemplate<{ record: ExternalUpstream }>()
const [DefineExternalSource, ExternalSource] = createReusableTemplate<{ record: ExternalUpstream }>()

// Search by upstream name or server address. The input is debounced into the
// `q` route query, so a filtered view survives a reload and can be shared.
const searchQuery = useRouteQuery<string>('q', '')
const searchText = ref(searchQuery.value)
watchDebounced(searchText, value => {
  searchQuery.value = value.trim()
}, { debounce: 250 })
watch(searchQuery, value => {
  if (value !== searchText.value.trim())
    searchText.value = value
})

const isSearching = computed(() => normalizeKeyword(searchQuery.value) !== '')
const filteredUpstreams = computed(() => filterUpstreams(upstreams.value, searchQuery.value))
const filteredExternal = computed(() => filterUpstreams(externalUpstreams.value, searchQuery.value))
const matchSummary = computed(() => {
  const groups = filteredUpstreams.value.length + filteredExternal.value.length
  const servers = [...filteredUpstreams.value, ...filteredExternal.value]
    .reduce((total, group) => total + group.servers.length, 0)
  return $gettext('Matched upstreams: %{groups}, servers: %{servers}', {
    groups: String(groups),
    servers: String(servers),
  })
})

async function loadData() {
  isLoading.value = true
  try {
    const res = await upstream.getManagedList()
    upstreams.value = res.data ?? []
    externalUpstreams.value = res.external ?? []
    managedDir.value = res.dir
  }
  finally {
    isLoading.value = false
  }
}

onMounted(loadData)

function openCreate() {
  editingName.value = ''
  isEditorOpen.value = true
}

function openEdit(name: string) {
  editingName.value = name
  isEditorOpen.value = true
}

// An upstream block inside a site can become a shared group of the same name.
const convertTarget = ref<ExternalUpstream>()
const isConvertOpen = ref(false)

function canConvert(record: ExternalUpstream) {
  return record.source.type === 'site' && !record.read_only
}

function openConvert(record: ExternalUpstream) {
  convertTarget.value = record
  isConvertOpen.value = true
}

type ServerStatus = 'success' | 'error' | 'default' | 'warning'

// Both tables look health up by the socket the backend resolved for each
// server, the key the health checker uses; see serverHealth.
const healthBadges: Record<ServerHealthState, ServerStatus> = {
  disabled: 'default',
  unknown: 'default',
  online: 'success',
  offline: 'error',
}

function serverStatus(isDown: boolean, socket: string | undefined): ServerStatus {
  return healthBadges[serverHealth(isDown, socket, proxyAvailabilityStore.availabilityResults).state]
}

function serverTitle(isDown: boolean, socket: string | undefined) {
  const health = serverHealth(isDown, socket, proxyAvailabilityStore.availabilityResults)
  switch (health.state) {
    case 'disabled':
      return $gettext('Disabled')
    case 'online':
      return `${$gettext('Online')} · ${health.result!.latency.toFixed(2)}ms`
    case 'offline':
      return $gettext('Offline')
    default:
      return $gettext('No Data')
  }
}

const sourceColors: Record<UpstreamSource['type'], string> = {
  managed: 'green',
  site: 'blue',
  stream: 'purple',
  config: 'default',
}

function externalRowKey(record: ExternalUpstream) {
  return `${record.config_path}\n${record.name}`
}

const sourceLabels: Record<UpstreamSource['type'], () => string> = {
  managed: () => $gettext('Upstream Group'),
  site: () => $gettext('Site'),
  stream: () => $gettext('Stream'),
  config: () => $gettext('Config'),
}

function sourceLink(source: UpstreamSource): RouteLocationRaw | undefined {
  switch (source.type) {
    case 'site':
      return `/sites/${encodeURIComponent(source.name)}`
    case 'stream':
      return `/streams/${encodeURIComponent(source.name)}`
    case 'config':
      if (source.name.startsWith('/'))
        return undefined
      return `/config/${source.name.split('/').map(encodeURIComponent).join('/')}/edit`
  }
  return undefined
}

// A server switch targets one `server` line of one upstream block in one file.
interface ServerTarget {
  upstream: string
  configPath: string
}

const togglingKeys = reactive(new Set<string>())

function toggleKey(target: ServerTarget, address: string) {
  return `${target.configPath}\n${target.upstream}\n${address}`
}

function isToggling(target: ServerTarget, address: string) {
  return togglingKeys.has(toggleKey(target, address))
}

function findManaged(target: ServerTarget) {
  return upstreams.value.find(item => item.name === target.upstream && item.path === target.configPath)
}

function findExternal(target: ServerTarget) {
  return externalUpstreams.value.find(item => item.name === target.upstream && item.config_path === target.configPath)
}

// The unfiltered server list of the group, so the last-server guard is not
// fooled by a search that hides the other servers.
function allServers(target: ServerTarget): Array<ManagedUpstreamServer | UpstreamServerState> {
  return findManaged(target)?.servers ?? findExternal(target)?.servers ?? []
}

function applyState(target: ServerTarget, state: UpstreamGroupState) {
  const managedGroup = findManaged(target)
  if (managedGroup) {
    for (const server of managedGroup.servers) {
      const updated = state.servers.find(item => item.address === server.address)
      if (updated)
        server.down = updated.down
    }
    return
  }
  const externalGroup = findExternal(target)
  if (externalGroup)
    externalGroup.servers = state.servers
}

async function submitToggle(target: ServerTarget, address: string, isEnabled: boolean) {
  const key = toggleKey(target, address)
  if (togglingKeys.has(key))
    return
  togglingKeys.add(key)
  try {
    const state = await upstream.setServerState({
      upstream: target.upstream,
      config_path: target.configPath,
      address,
      enabled: isEnabled,
    })
    applyState(target, state)
    message.success(isEnabled
      ? $gettext('Server %{address} enabled', { address })
      : $gettext('Server %{address} disabled', { address }))
  }
  catch {
    // The request layer already shows the translated error (for example the
    // nginx -t output); the file was rolled back, so the switch keeps its
    // previous position.
  }
  finally {
    togglingKeys.delete(key)
  }
}

function requestToggle(target: ServerTarget, address: string, isEnabled: boolean) {
  if (!isEnabled && disablesLastPrimary(allServers(target), address)) {
    modal.confirm({
      title: $gettext('Disable the last active server of %{name}?', { name: target.upstream }),
      content: $gettext('No other primary server of this upstream is enabled. Requests will fail unless a backup server takes over.'),
      okText: $gettext('Disable'),
      okButtonProps: { danger: true },
      cancelText: $gettext('Cancel'),
      onOk: () => submitToggle(target, address, isEnabled),
    })
    return
  }
  submitToggle(target, address, isEnabled)
}

function openReference(reference: ManagedUpstreamDetail['references'][number]) {
  if (reference.type === 'site')
    router.push(`/sites/${encodeURIComponent(reference.name)}`)
}

function confirmDelete(record: ManagedUpstreamDetail) {
  // nginx cannot resolve a proxy_pass to a missing group, so a referenced
  // upstream is never deleted; the backend refuses it too.
  if (record.references.length > 0) {
    modal.warning({
      title: $gettext('Upstream %{name} is still in use', { name: record.name }),
      content: h('div', [
        h('p', $gettext('Point these configurations at another upstream before deleting it:')),
        h('ul', { 'class': 'pl-5 mb-0', 'data-testid': 'upstream-delete-references' }, record.references.map(reference => h('li', { key: reference.path }, reference.name))),
      ]),
      okText: $gettext('OK'),
    })
    return
  }

  modal.confirm({
    title: $gettext('Delete upstream %{name}?', { name: record.name }),
    content: $gettext('%{path} will be removed and Nginx reloaded.', { path: record.path }),
    okText: $gettext('Delete'),
    okButtonProps: { danger: true },
    cancelText: $gettext('Cancel'),
    async onOk() {
      await upstream.deleteManaged(record.name)
      message.success($gettext('Upstream %{name} deleted', { name: record.name }))
      await loadData()
    },
  })
}
</script>

<template>
  <ACard :title="$gettext('Upstream Groups')">
    <!-- Cell fragments shared by the full and the compact layout. -->
    <DefineManagedServers v-slot="{ record }">
      <div class="flex flex-col gap-1">
        <UpstreamServerLine
          v-for="server in record.servers"
          :key="server.address"
          :address="server.address"
          :is-down="server.down"
          :is-backup="server.backup"
          :weight="server.weight"
          :status="serverStatus(server.down, server.socket)"
          :status-title="serverTitle(server.down, server.socket)"
          :is-toggling="isToggling({ upstream: record.name, configPath: record.path }, server.address)"
          @toggle="isEnabled => requestToggle({ upstream: record.name, configPath: record.path }, server.address, isEnabled)"
        />
      </div>
    </DefineManagedServers>

    <DefineManagedActions v-slot="{ record }">
      <ASpace :size="0">
        <AButton
          type="link"
          size="small"
          :data-testid="`upstream-edit-${record.name}`"
          @click="openEdit(record.name)"
        >
          {{ $gettext('Edit') }}
        </AButton>
        <AButton
          type="link"
          size="small"
          danger
          :data-testid="`upstream-delete-${record.name}`"
          @click="confirmDelete(record)"
        >
          {{ $gettext('Delete') }}
        </AButton>
      </ASpace>
    </DefineManagedActions>

    <DefineExternalServers v-slot="{ record }">
      <div class="flex flex-col gap-1">
        <UpstreamServerLine
          v-for="server in record.servers"
          :key="server.address"
          :address="server.address"
          :is-down="server.down"
          :is-backup="server.backup"
          :weight="server.weight"
          :status="serverStatus(server.down, server.socket)"
          :status-title="serverTitle(server.down, server.socket)"
          :is-read-only="record.read_only"
          :is-toggling="isToggling({ upstream: record.name, configPath: record.config_path }, server.address)"
          @toggle="isEnabled => requestToggle({ upstream: record.name, configPath: record.config_path }, server.address, isEnabled)"
        />
      </div>
    </DefineExternalServers>

    <DefineExternalSource v-slot="{ record }">
      <div class="flex flex-wrap items-center gap-1">
        <ATag
          :color="sourceColors[record.source.type]"
          class="me-0"
        >
          {{ sourceLabels[record.source.type]?.() ?? record.source.type }}
        </ATag>
        <RouterLink
          v-if="sourceLink(record.source)"
          :to="sourceLink(record.source)!"
          class="break-all"
        >
          {{ record.source.name }}
        </RouterLink>
        <span
          v-else
          class="font-mono break-all"
        >
          {{ record.config_path }}
        </span>
        <AButton
          v-if="canConvert(record)"
          type="link"
          size="small"
          :data-testid="`upstream-convert-${record.name}`"
          @click="openConvert(record)"
        >
          {{ $gettext('Convert to Shared Group') }}
        </AButton>
      </div>
    </DefineExternalSource>

    <template #extra>
      <ASpace>
        <AButton
          :loading="isLoading"
          :aria-label="$gettext('Reload')"
          @click="loadData"
        >
          <template #icon>
            <ReloadOutlined />
          </template>
        </AButton>
        <AButton
          type="primary"
          data-testid="upstream-create"
          @click="openCreate"
        >
          <template #icon>
            <PlusOutlined />
          </template>
          {{ $gettext('Create Upstream') }}
        </AButton>
      </ASpace>
    </template>

    <p class="mt-0 mb-4 text-gray-500 dark:text-gray-400">
      {{ $gettext('An upstream group shares its servers and load balancing method across sites. Choose it as the proxy target when setting up a site; later changes here apply to every site that uses it.') }}
      <template v-if="managedDir">
        {{ $gettext('Files are stored in %{dir}.', { dir: managedDir }) }}
      </template>
    </p>

    <AFlex
      wrap
      gap="small"
      align="center"
      class="mb-4"
    >
      <AInput
        v-model:value="searchText"
        allow-clear
        class="w-full sm:max-w-100"
        :placeholder="$gettext('Search by name or IP:port')"
        :aria-label="$gettext('Search upstreams by name or server address')"
        data-testid="upstream-search"
      >
        <template #prefix>
          <SearchOutlined class="text-gray-400" />
        </template>
      </AInput>
      <span
        v-if="isSearching"
        class="text-gray-500 dark:text-gray-400"
        data-testid="upstream-search-summary"
      >
        {{ matchSummary }}
      </span>
    </AFlex>

    <ATable
      :columns="columns"
      :data-source="filteredUpstreams"
      :loading="isLoading"
      :locale="isSearching ? { emptyText: $gettext('No upstream group matches the search') } : undefined"
      :pagination="false"
      :scroll="isNarrow ? undefined : { x: 'max-content' }"
      row-key="name"
      data-testid="upstream-table"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'compact'">
          <div class="flex flex-col gap-2">
            <div class="flex flex-wrap items-center justify-between gap-1">
              <span class="font-mono font-medium break-all">{{ record.name }}</span>
              <ManagedActions :record />
            </div>
            <ManagedServers :record />
          </div>
        </template>

        <template v-else-if="column.key === 'name'">
          <span class="font-mono font-medium">{{ record.name }}</span>
        </template>

        <template v-else-if="column.key === 'method'">
          <div class="flex flex-wrap items-center gap-1">
            <span>{{ methodLabels[record.method]?.() ?? record.method }}</span>
            <ATag
              v-if="record.method === 'hash'"
              class="font-mono"
            >
              {{ record.hash_key }}{{ record.consistent ? ' consistent' : '' }}
            </ATag>
            <ATag v-if="record.keepalive > 0">
              keepalive {{ record.keepalive }}
            </ATag>
            <ATag
              v-if="record.zone"
              :title="$gettext('Shared Memory Zone')"
            >
              zone {{ record.zone_size }}
            </ATag>
          </div>
        </template>

        <template v-else-if="column.key === 'servers'">
          <ManagedServers :record />
        </template>

        <template v-else-if="column.key === 'references'">
          <div
            v-if="record.references.length > 0"
            class="flex flex-wrap gap-1"
          >
            <ATag
              v-for="reference in record.references"
              :key="reference.path"
              :color="reference.type === 'site' ? 'blue' : 'default'"
              :class="{ 'cursor-pointer': reference.type === 'site' }"
              @click="openReference(reference)"
            >
              {{ reference.name }}
            </ATag>
          </div>
          <span
            v-else
            class="text-gray-500 dark:text-gray-400"
          >
            {{ $gettext('Not used') }}
          </span>
        </template>

        <template v-else-if="column.key === 'actions'">
          <ManagedActions :record />
        </template>
      </template>
    </ATable>

    <template v-if="filteredExternal.length > 0">
      <h3 class="mt-8 mb-1">
        {{ $gettext('Upstreams Defined in Other Files') }}
      </h3>
      <p class="mt-0 mb-4 text-gray-500 dark:text-gray-400">
        {{ $gettext('These upstream blocks live inside site or other configuration files. Edit their settings in their own file; servers can be switched on and off here. A block inside a site can be converted to a shared group that other sites can use too.') }}
      </p>
      <ATable
        :columns="externalColumns"
        :data-source="filteredExternal"
        :pagination="false"
        :scroll="isNarrow ? undefined : { x: 'max-content' }"
        :row-key="externalRowKey"
        size="small"
        data-testid="upstream-external-table"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'compact'">
            <div class="flex flex-col gap-2">
              <div class="flex flex-wrap items-center justify-between gap-1">
                <span class="font-mono font-medium break-all">{{ record.name }}</span>
                <ExternalSource :record />
              </div>
              <ExternalServers :record />
            </div>
          </template>

          <template v-else-if="column.key === 'name'">
            <span class="font-mono font-medium">{{ record.name }}</span>
          </template>

          <template v-else-if="column.key === 'servers'">
            <ExternalServers :record />
          </template>

          <template v-else-if="column.key === 'source'">
            <ExternalSource :record />
          </template>
        </template>
      </ATable>
    </template>

    <UpstreamEditor
      v-model:open="isEditorOpen"
      :name="editingName"
      @saved="loadData"
    />

    <ConvertUpstreamModal
      v-if="convertTarget"
      v-model:open="isConvertOpen"
      :site="convertTarget.source.name"
      :upstream="convertTarget.name"
      @converted="loadData"
    />
  </ACard>
</template>
