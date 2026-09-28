<script setup lang="ts">
import type { TableColumnsType } from 'antdv-next'
import type { ExternalUpstream, ManagedUpstreamDetail, ManagedUpstreamServer } from '@/api/upstream'
import { PlusOutlined, ReloadOutlined } from '@antdv-next/icons'
import upstream from '@/api/upstream'
import { useProxyAvailability } from '@/composables/useProxyAvailability'
import UpstreamEditor from './components/UpstreamEditor.vue'

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

const columns: TableColumnsType<ManagedUpstreamDetail> = [
  { title: () => $gettext('Name'), key: 'name', width: 180 },
  { title: () => $gettext('Load Balancing'), key: 'method', width: 200 },
  { title: () => $gettext('Servers'), key: 'servers' },
  { title: () => $gettext('Referenced By'), key: 'references', width: 240 },
  { title: () => $gettext('Actions'), key: 'actions', width: 150, fixed: 'right' },
]

const externalColumns: TableColumnsType<ExternalUpstream> = [
  { title: () => $gettext('Name'), dataIndex: 'name', key: 'name', width: 180 },
  {
    title: () => $gettext('Servers'),
    key: 'servers',
    render: (_value, record) => record.servers.map(server => `${server.host}:${server.port}`).join(', '),
  },
  { title: () => $gettext('Defined In'), dataIndex: 'config_path', key: 'config_path' },
]

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

type ServerStatus = 'success' | 'error' | 'default' | 'warning'

function serverStatus(server: ManagedUpstreamServer): ServerStatus {
  if (server.down)
    return 'default'
  const result = proxyAvailabilityStore.availabilityResults[server.address]
  if (!result)
    return 'default'
  return result.online ? 'success' : 'error'
}

function serverTitle(server: ManagedUpstreamServer) {
  if (server.down)
    return $gettext('Disabled')
  const result = proxyAvailabilityStore.availabilityResults[server.address]
  if (!result)
    return $gettext('No Data')
  return result.online
    ? `${$gettext('Online')} · ${result.latency.toFixed(2)}ms`
    : $gettext('Offline')
}

function serverFlags(server: ManagedUpstreamServer) {
  const flags: string[] = []
  if (server.weight && server.weight !== 1)
    flags.push(`weight=${server.weight}`)
  if (server.backup)
    flags.push('backup')
  if (server.down)
    flags.push('down')
  return flags
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

    <ATable
      :columns="columns"
      :data-source="upstreams"
      :loading="isLoading"
      :pagination="false"
      :scroll="{ x: 1000 }"
      row-key="name"
      data-testid="upstream-table"
    >
      <template #bodyCell="{ column, record }">
        <template v-if="column.key === 'name'">
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
          <div class="flex flex-col gap-1">
            <div
              v-for="server in record.servers"
              :key="server.address"
              class="flex flex-wrap items-center gap-1"
              :title="serverTitle(server)"
            >
              <ABadge
                :status="serverStatus(server)"
                :text="server.address"
                :class="{ 'line-through opacity-60': server.down }"
                class="font-mono"
              />
              <ATag
                v-for="flag in serverFlags(server)"
                :key="flag"
                :color="flag === 'down' ? 'default' : flag === 'backup' ? 'orange' : 'blue'"
              >
                {{ flag }}
              </ATag>
            </div>
          </div>
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
        </template>
      </template>
    </ATable>

    <template v-if="externalUpstreams.length > 0">
      <h3 class="mt-8 mb-1">
        {{ $gettext('Upstreams Defined in Other Files') }}
      </h3>
      <p class="mt-0 mb-4 text-gray-500 dark:text-gray-400">
        {{ $gettext('These upstream blocks live inside site or other configuration files. Edit them in their own file.') }}
      </p>
      <ATable
        :columns="externalColumns"
        :data-source="externalUpstreams"
        :pagination="false"
        :scroll="{ x: 800 }"
        row-key="name"
        size="small"
      />
    </template>

    <UpstreamEditor
      v-model:open="isEditorOpen"
      :name="editingName"
      @saved="loadData"
    />
  </ACard>
</template>
