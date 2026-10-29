<script setup lang="ts">
import type { SelectProps, TableColumnsType } from 'antdv-next'
import type { BackendHealthSummary } from './ingressStatus'
import type { Ingress } from '@/api/ingress'
import { ReloadOutlined, SearchOutlined } from '@antdv-next/icons'
import { breakpointsAntDesign, createReusableTemplate, useBreakpoints, useDocumentVisibility, useIntervalFn, watchDebounced } from '@vueuse/core'
import { useRouteQuery } from '@vueuse/router'
import { Checkbox } from 'antdv-next'
import ingressApi from '@/api/ingress'
import { formatDateTime, fromNow } from '@/lib/helper'
import IngressCertExpiry from './components/IngressCertExpiry.vue'
import IngressDetail from './components/IngressDetail.vue'
import IngressPhaseTag from './components/IngressPhaseTag.vue'
import { backendHealth, backendHealthColor, filterIngresses, hostUrl, isGenerationBehind, ownersOf } from './ingressStatus'

const { message, modal } = App.useApp()

const docsUrl = 'https://nginxui.com/guide/ingress.html'

const ingresses = ref<Ingress[]>([])
const isLoading = ref(false)
const hasLoaded = ref(false)

// Owner, search and the open ingress live in the route query, so a filtered
// view or an ingress opened from a managed site survives a reload.
const ownerQuery = useRouteQuery<string>('owner', '')
const searchQuery = useRouteQuery<string>('q', '')
const openName = useRouteQuery<string>('name', '')

const searchText = ref(searchQuery.value)
watchDebounced(searchText, value => {
  searchQuery.value = value.trim()
}, { debounce: 250 })

const ownerOptions = computed<SelectProps['options']>(() => [
  {
    value: '',
    label: $gettext('All owners'),
  },
  ...ownersOf(ingresses.value).map(owner => ({ value: owner, label: owner })),
])

const filteredIngresses = computed(() => filterIngresses(ingresses.value, ownerQuery.value, searchQuery.value))
const isFiltering = computed(() => !!ownerQuery.value || !!searchQuery.value)

// The list is small and the controller usually changes it in bursts, so the
// whole list is fetched and filtered here. `silent` refreshes run in the
// background and keep errors out of the toast stack.
async function loadData(silent = false) {
  if (!silent)
    isLoading.value = true
  try {
    const res = await ingressApi.getList(undefined, silent
      ? { skipErrHandling: true }
      : undefined)
    ingresses.value = res.data ?? []
    hasLoaded.value = true
  }
  catch {
    // The global handler reports a foreground failure; a background one is
    // retried on the next tick.
  }
  finally {
    if (!silent)
      isLoading.value = false
  }
}

onMounted(() => loadData())

// Applies are asynchronous: poll while the page is visible so phases move from
// Pending to Ready without a manual reload.
const visibility = useDocumentVisibility()
const { pause, resume } = useIntervalFn(() => loadData(true), 10_000)
watch(visibility, value => value === 'visible'
  ? resume()
  : pause(), { immediate: true })

const selectedIngress = computed(() => ingresses.value.find(ingress => ingress.name === openName.value))
const isDetailOpen = computed({
  get: () => !!openName.value && !!selectedIngress.value,
  set: value => {
    if (!value)
      openName.value = ''
  },
})

function openDetail(record: Ingress) {
  openName.value = record.name
}

const reconciling = ref<Record<string, boolean>>({})

async function reconcile(record: Ingress) {
  reconciling.value[record.name] = true
  try {
    const updated = await ingressApi.reconcile(record.name)
    if (updated?.name) {
      ingresses.value = ingresses.value.map(ingress => ingress.name === updated.name
        ? updated
        : ingress)
    }
    message.success($gettext('Reconcile of %{name} queued', { name: record.name }))
  }
  finally {
    reconciling.value[record.name] = false
  }
}

// Ingresses belong to their controller, which usually recreates a deleted one
// on its next sync. The confirmation says so and the request names the owner
// and forces the delete, so it also works when the owner is unknown here.
function confirmDelete(record: Ingress) {
  const purgeCert = ref(false)
  modal.confirm({
    title: $gettext('Delete ingress %{name}?', { name: record.name }),
    content: () => h('div', [
      h('p', $gettext('This ingress is owned by %{owner}. Its controller will probably recreate it on the next sync; remove it from the controller to delete it for good.', { owner: record.owner })),
      h('p', $gettext('The generated site and upstream files are removed and nginx is reloaded.')),
      record.spec?.tls?.mode === 'acme'
        ? h(Checkbox, {
            'checked': purgeCert.value,
            'onUpdate:checked': (value: boolean) => { purgeCert.value = value },
          }, () => $gettext('Also delete the certificate'))
        : null,
    ]),
    okText: $gettext('Delete'),
    okButtonProps: { danger: true },
    cancelText: $gettext('Cancel'),
    async onOk() {
      await ingressApi.deleteItem(record.name, {
        owner: record.owner,
        force: true,
        purge_cert: purgeCert.value,
      })
      message.success($gettext('Ingress %{name} deleted', { name: record.name }))
      if (openName.value === record.name)
        openName.value = ''
      await loadData()
    },
  })
}

const isNarrow = useBreakpoints(breakpointsAntDesign).smaller('md')

const columns = computed<TableColumnsType<Ingress>>(() => isNarrow.value
  ? [
      {
        title: () => $gettext('Ingress'),
        key: 'compact',
      },
    ]
  : [
      // The owner sits under the name and the generation under the phase, so
      // the table fits a laptop screen without scrolling sideways.
      {
        title: () => $gettext('Name'),
        key: 'name',
        width: 220,
      },
      {
        title: () => $gettext('Hosts'),
        key: 'hosts',
        width: 220,
      },
      {
        title: () => $gettext('Phase'),
        key: 'phase',
        width: 120,
      },
      {
        title: () => $gettext('Certificate'),
        key: 'cert',
        width: 180,
      },
      {
        title: () => $gettext('Backends'),
        key: 'backends',
        width: 140,
      },
      {
        title: () => $gettext('Updated at'),
        key: 'updated_at',
        width: 110,
      },
      {
        title: () => $gettext('Actions'),
        key: 'actions',
        width: 150,
        fixed: 'right',
      },
    ])

const [DefineHosts, Hosts] = createReusableTemplate<{ record: Ingress }>()
const [DefineBackends, Backends] = createReusableTemplate<{ health: BackendHealthSummary }>()
const [DefineActions, Actions] = createReusableTemplate<{ record: Ingress }>()
const [DefineOwner, Owner] = createReusableTemplate<{ record: Ingress }>()
const [DefineGeneration, Generation] = createReusableTemplate<{ record: Ingress }>()
</script>

<template>
  <ACard :title="$gettext('Ingresses')">
    <DefineHosts v-slot="{ record }">
      <AFlex
        vertical
        align="flex-start"
        gap="small"
      >
        <a
          v-for="host in record.spec?.hosts ?? []"
          :key="host"
          :href="hostUrl(record, host)"
          target="_blank"
          rel="noopener noreferrer"
          @click.stop
        >
          <ATag color="blue" class="me-0 break-all whitespace-normal">{{ host }}</ATag>
        </a>
      </AFlex>
    </DefineHosts>

    <DefineBackends v-slot="{ health }">
      <AFlex
        wrap
        align="center"
        gap="small"
        data-testid="ingress-backend-health"
      >
        <ABadge
          v-if="health.known"
          :status="backendHealthColor(health)"
          :text="$gettext('%{healthy}/%{total} healthy', {
            healthy: String(health.healthy),
            total: String(health.total),
          })"
        />
        <span v-else class="text-gray-500 dark:text-gray-400">
          {{ $ngettext('%{total} server', '%{total} servers', health.total, { total: String(health.total) }) }}
        </span>
        <ATag
          v-if="health.down > 0"
          class="me-0"
        >
          {{ $gettext('%{count} down', { count: String(health.down) }) }}
        </ATag>
      </AFlex>
    </DefineBackends>

    <DefineOwner v-slot="{ record }">
      <ATag
        class="me-0 cursor-pointer"
        :title="$gettext('Filter by owner')"
        data-testid="ingress-owner"
        @click="ownerQuery = record.owner"
      >
        {{ record.owner }}
      </ATag>
    </DefineOwner>

    <DefineGeneration v-slot="{ record }">
      <ATooltip :title="$gettext('Applied generation / latest generation')">
        <span
          class="text-xs font-mono"
          :class="isGenerationBehind(record) ? 'text-orange-500' : 'text-gray-500 dark:text-gray-400'"
          data-testid="ingress-generation"
        >
          {{ $gettext('gen %{observed}/%{generation}', {
            observed: String(record.status?.observed_generation ?? 0),
            generation: String(record.generation),
          }) }}
        </span>
      </ATooltip>
    </DefineGeneration>

    <DefineActions v-slot="{ record }">
      <ASpace :size="0" wrap>
        <AButton
          type="link"
          size="small"
          :data-testid="`ingress-view-${record.name}`"
          @click="openDetail(record)"
        >
          {{ $gettext('Details') }}
        </AButton>
        <RouterLink
          v-if="record.site_path"
          v-slot="{ navigate }"
          :to="`/sites/${encodeURIComponent(record.site_path)}`"
          custom
        >
          <AButton
            type="link"
            size="small"
            @click="navigate"
          >
            {{ $gettext('Site') }}
          </AButton>
        </RouterLink>
        <AButton
          type="link"
          size="small"
          :loading="reconciling[record.name]"
          @click="reconcile(record)"
        >
          {{ $gettext('Reconcile') }}
        </AButton>
      </ASpace>
    </DefineActions>

    <template #extra>
      <AButton
        :loading="isLoading"
        :aria-label="$gettext('Reload')"
        @click="loadData()"
      >
        <template #icon>
          <ReloadOutlined />
        </template>
      </AButton>
    </template>

    <p class="mt-0 mb-4 text-gray-500 dark:text-gray-400">
      {{ $gettext('Ingresses are reverse proxy entries published by external controllers through the Ingress API. Nginx UI renders each one into a managed site and upstreams, issues its certificate and reports its status here. Change them in their controller; edits made here are overwritten by the next reconcile.') }}
      <a
        :href="docsUrl"
        target="_blank"
        rel="noopener noreferrer"
      >{{ $gettext('Learn more') }}</a>
    </p>

    <template v-if="hasLoaded && ingresses.length === 0">
      <AEmpty
        class="my-12"
        data-testid="ingress-empty"
      >
        <template #description>
          <div class="max-w-120 mx-auto">
            <div class="mb-2 font-medium">
              {{ $gettext('No ingresses yet') }}
            </div>
            <div class="text-gray-500 dark:text-gray-400">
              {{ $gettext('A controller such as a deployment platform creates ingresses with PUT /api/ingresses/:name, authenticated as a cluster node or with an API token. They show up here as soon as they are stored.') }}
            </div>
          </div>
        </template>
        <AButton
          type="primary"
          :href="docsUrl"
          target="_blank"
          rel="noopener noreferrer"
        >
          {{ $gettext('Read the Ingress API guide') }}
        </AButton>
      </AEmpty>
    </template>

    <template v-else>
      <AFlex
        wrap
        gap="small"
        align="center"
        class="mb-4"
      >
        <AInput
          v-model:value="searchText"
          allow-clear
          class="w-full sm:max-w-80"
          :placeholder="$gettext('Search by name, host or external ID')"
          :aria-label="$gettext('Search ingresses')"
          data-testid="ingress-search"
        >
          <template #prefix>
            <SearchOutlined class="text-gray-400" />
          </template>
        </AInput>
        <ASelect
          v-model:value="ownerQuery"
          class="w-full sm:w-60"
          :options="ownerOptions"
          :aria-label="$gettext('Filter by owner')"
          data-testid="ingress-owner-filter"
        />
      </AFlex>

      <ATable
        :columns="columns"
        :data-source="filteredIngresses"
        :loading="isLoading"
        :locale="isFiltering ? { emptyText: $gettext('No ingress matches the filter') } : undefined"
        :pagination="false"
        :scroll="isNarrow ? undefined : { x: 'max-content' }"
        row-key="name"
        data-testid="ingress-table"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'compact'">
            <div class="flex flex-col gap-2">
              <AFlex
                wrap
                align="center"
                justify="space-between"
                gap="small"
              >
                <a
                  class="font-mono font-medium break-all"
                  @click="openDetail(record)"
                >{{ record.name }}</a>
                <AFlex align="center" gap="small">
                  <Generation :record />
                  <IngressPhaseTag :ingress="record" />
                </AFlex>
              </AFlex>
              <div><Owner :record /></div>
              <Hosts :record />
              <Backends :health="backendHealth(record)" />
              <div><IngressCertExpiry :ingress="record" /></div>
              <Actions :record />
            </div>
          </template>

          <template v-else-if="column.key === 'name'">
            <div class="flex flex-col items-start gap-1">
              <a
                class="font-mono font-medium break-all"
                @click="openDetail(record)"
              >{{ record.name }}</a>
              <span
                v-if="record.external_id"
                class="text-xs text-gray-500 dark:text-gray-400 font-mono break-all"
              >{{ record.external_id }}</span>
              <Owner :record />
            </div>
          </template>

          <template v-else-if="column.key === 'hosts'">
            <Hosts :record />
          </template>

          <template v-else-if="column.key === 'phase'">
            <div class="flex flex-col items-start gap-1">
              <IngressPhaseTag :ingress="record" />
              <Generation :record />
            </div>
          </template>

          <template v-else-if="column.key === 'cert'">
            <IngressCertExpiry :ingress="record" />
          </template>

          <template v-else-if="column.key === 'backends'">
            <Backends :health="backendHealth(record)" />
          </template>

          <template v-else-if="column.key === 'updated_at'">
            <span :title="formatDateTime(record.updated_at)">{{ fromNow(record.updated_at) }}</span>
          </template>

          <template v-else-if="column.key === 'actions'">
            <Actions :record />
          </template>
        </template>
      </ATable>
    </template>

    <IngressDetail
      v-model:open="isDetailOpen"
      :ingress="selectedIngress"
      :is-reconciling="selectedIngress ? reconciling[selectedIngress.name] : false"
      @reconcile="reconcile"
      @delete="confirmDelete"
    />
  </ACard>
</template>
