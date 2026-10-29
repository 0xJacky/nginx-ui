<script setup lang="ts">
import type { TableColumnsType } from 'antdv-next'
import type { Ingress, IngressBackend, IngressCondition, IngressRoute, IngressServer } from '@/api/ingress'
import { ReloadOutlined } from '@antdv-next/icons'
import { breakpointsAntDesign, useBreakpoints } from '@vueuse/core'
import site from '@/api/site'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import { formatDateTime } from '@/lib/helper'
import { certExpiry, hostUrl, isGenerationBehind, serverStatus } from '../ingressStatus'
import IngressCertExpiry from './IngressCertExpiry.vue'
import IngressPhaseTag from './IngressPhaseTag.vue'
import { challengeLabel, conditionLabel, lbLabel, tlsModeLabel } from './labels'

const props = defineProps<{
  ingress?: Ingress
  isReconciling?: boolean
}>()

const emit = defineEmits<{
  reconcile: [ingress: Ingress]
  delete: [ingress: Ingress]
}>()

const open = defineModel<boolean>('open', { default: false })

const isNarrow = useBreakpoints(breakpointsAntDesign).smaller('md')
const drawerSize = computed(() => isNarrow.value ? window.innerWidth : 820)

const siteLink = computed(() => props.ingress?.site_path
  ? `/sites/${encodeURIComponent(props.ingress.site_path)}`
  : '')

const labels = computed(() => Object.entries(props.ingress?.labels ?? {}))

// The failing apply keeps the previous config live; its message carries the
// nginx -t output.
const applyError = computed(() => props.ingress?.status?.conditions
  ?.find(condition => condition.type === 'ConfigApplied' && !condition.status))

const cert = computed(() => props.ingress
  ? certExpiry(props.ingress)
  : undefined)

const serverColumns: TableColumnsType<IngressServer> = [
  {
    title: () => $gettext('Address'),
    key: 'address',
  },
  {
    title: () => $gettext('Health'),
    key: 'health',
    width: 120,
  },
  {
    title: () => $gettext('Weight'),
    key: 'weight',
    width: 80,
  },
  {
    title: () => $gettext('Max Fails'),
    key: 'max_fails',
    width: 90,
  },
  {
    title: () => $gettext('Fail Timeout'),
    key: 'fail_timeout',
    width: 110,
  },
]

const routeColumns: TableColumnsType<IngressRoute> = [
  {
    title: () => $gettext('Path Prefix'),
    key: 'path_prefix',
  },
  {
    title: () => $gettext('Backend'),
    key: 'backend',
  },
  {
    title: () => $gettext('Options'),
    key: 'options',
  },
  {
    title: () => $gettext('Allowed Sources'),
    key: 'allow_cidrs',
  },
]

const conditionColumns: TableColumnsType<IngressCondition> = [
  {
    title: () => $gettext('Type'),
    key: 'type',
    width: 130,
  },
  {
    title: () => $gettext('Status'),
    key: 'status',
    width: 140,
  },
  {
    title: () => $gettext('Message'),
    key: 'message',
  },
  {
    title: () => $gettext('Last Transition'),
    key: 'last_transition_time',
    width: 140,
  },
]

function health(backend: IngressBackend, server: IngressServer) {
  if (!props.ingress)
    return undefined
  return serverStatus(props.ingress, backend.name, server.address)
}

function healthBadge(backend: IngressBackend, server: IngressServer): { status: 'success' | 'error' | 'default', text: string } {
  if (server.down) {
    return {
      status: 'default',
      text: $gettext('Drained'),
    }
  }
  const status = health(backend, server)
  if (!status?.checked) {
    return {
      status: 'default',
      text: $gettext('No Data'),
    }
  }
  return status.healthy
    ? {
        status: 'success',
        text: $gettext('Healthy'),
      }
    : {
        status: 'error',
        text: $gettext('Unhealthy'),
      }
}

// The generated site file is loaded on demand from the regular site API.
const generatedConfig = ref('')
const isConfigLoading = ref(false)
const configError = ref('')
const activePanels = ref<string[]>([])

async function loadGeneratedConfig() {
  if (!props.ingress?.site_path)
    return
  isConfigLoading.value = true
  configError.value = ''
  try {
    const res = await site.getItem(encodeURIComponent(props.ingress.site_path), undefined, { skipErrHandling: true })
    generatedConfig.value = res.config ?? ''
  }
  catch {
    configError.value = $gettext('The generated site could not be loaded. It is written by the first successful apply.')
  }
  finally {
    isConfigLoading.value = false
  }
}

watch(activePanels, panels => {
  if (panels.includes('config') && !generatedConfig.value && !isConfigLoading.value)
    loadGeneratedConfig()
})

watch(() => props.ingress?.name, () => {
  generatedConfig.value = ''
  configError.value = ''
  activePanels.value = []
})
</script>

<template>
  <ADrawer
    v-model:open="open"
    :size="drawerSize"
    destroy-on-hidden
    data-testid="ingress-detail"
  >
    <template #title>
      <AFlex
        v-if="ingress"
        wrap
        align="center"
        gap="small"
      >
        <span class="font-mono break-all">{{ ingress.name }}</span>
        <IngressPhaseTag :ingress />
      </AFlex>
    </template>
    <template #extra>
      <ASpace v-if="ingress">
        <AButton
          :loading="isReconciling"
          data-testid="ingress-reconcile"
          @click="emit('reconcile', ingress)"
        >
          <template #icon>
            <ReloadOutlined />
          </template>
          {{ $gettext('Reconcile Now') }}
        </AButton>
        <AButton
          danger
          data-testid="ingress-delete"
          @click="emit('delete', ingress)"
        >
          {{ $gettext('Delete') }}
        </AButton>
      </ASpace>
    </template>

    <template v-if="ingress">
      <AAlert
        v-if="applyError"
        type="error"
        show-icon
        class="mb-4"
        :title="$gettext('The last apply failed; the previous config is still live')"
      >
        <template #description>
          <div v-if="applyError.reason" class="mb-1">
            {{ applyError.reason }}
          </div>
          <pre
            v-if="applyError.message"
            class="m-0 max-h-60 overflow-auto whitespace-pre-wrap break-all text-xs"
          >{{ applyError.message }}</pre>
        </template>
      </AAlert>

      <ADescriptions
        bordered
        size="small"
        :column="isNarrow ? 1 : 2"
        class="mb-6"
      >
        <ADescriptionsItem :label="$gettext('Owner')">
          <ATag class="me-0">
            {{ ingress.owner }}
          </ATag>
        </ADescriptionsItem>
        <ADescriptionsItem :label="$gettext('External ID')">
          <span class="font-mono break-all">{{ ingress.external_id || '-' }}</span>
        </ADescriptionsItem>
        <ADescriptionsItem :label="$gettext('Generation')">
          <span :class="{ 'text-orange-500': isGenerationBehind(ingress) }">
            {{ $gettext('%{observed} applied of %{generation}', {
              observed: String(ingress.status?.observed_generation ?? 0),
              generation: String(ingress.generation),
            }) }}
          </span>
        </ADescriptionsItem>
        <ADescriptionsItem :label="$gettext('Generated Site')">
          <RouterLink
            v-if="siteLink"
            :to="siteLink"
            class="font-mono break-all"
          >
            {{ ingress.site_path }}
          </RouterLink>
          <span v-else>-</span>
        </ADescriptionsItem>
        <ADescriptionsItem :label="$gettext('Last Applied')">
          {{ ingress.status?.last_applied_at ? formatDateTime(ingress.status.last_applied_at) : '-' }}
        </ADescriptionsItem>
        <ADescriptionsItem :label="$gettext('Updated at')">
          {{ formatDateTime(ingress.updated_at) }}
        </ADescriptionsItem>
        <ADescriptionsItem :label="$gettext('Created at')">
          {{ formatDateTime(ingress.created_at) }}
        </ADescriptionsItem>
        <ADescriptionsItem :label="$gettext('Config Hash')">
          <span class="font-mono break-all text-xs">{{ ingress.status?.config_hash || '-' }}</span>
        </ADescriptionsItem>
        <ADescriptionsItem
          v-if="labels.length > 0"
          :label="$gettext('Labels')"
          :span="isNarrow ? 1 : 2"
        >
          <AFlex wrap gap="small">
            <ATag
              v-for="[key, value] in labels"
              :key
              class="me-0 font-mono"
            >
              {{ key }}={{ value }}
            </ATag>
          </AFlex>
        </ADescriptionsItem>
      </ADescriptions>

      <h3 class="mt-0 mb-2">
        {{ $gettext('Hosts') }}
      </h3>
      <AFlex
        wrap
        gap="small"
        class="mb-6"
      >
        <a
          v-for="host in ingress.spec?.hosts ?? []"
          :key="host"
          :href="hostUrl(ingress, host)"
          target="_blank"
          rel="noopener noreferrer"
        >
          <ATag color="blue" class="me-0">{{ host }}</ATag>
        </a>
      </AFlex>

      <h3 class="mt-0 mb-2">
        {{ $gettext('TLS') }}
      </h3>
      <ADescriptions
        bordered
        size="small"
        :column="isNarrow ? 1 : 2"
        class="mb-6"
      >
        <ADescriptionsItem :label="$gettext('Mode')">
          {{ tlsModeLabel(ingress.spec?.tls?.mode) }}
        </ADescriptionsItem>
        <ADescriptionsItem :label="$gettext('Certificate')">
          <AFlex wrap align="center" gap="small">
            <IngressCertExpiry :ingress />
            <RouterLink
              v-if="ingress.status?.cert_id"
              :to="`/certificates/${ingress.status.cert_id}`"
            >
              {{ $gettext('View certificate') }}
            </RouterLink>
          </AFlex>
        </ADescriptionsItem>
        <template v-if="ingress.spec?.tls?.mode === 'acme'">
          <ADescriptionsItem :label="$gettext('Challenge')">
            {{ challengeLabel(ingress.spec.tls.challenge) }}
          </ADescriptionsItem>
          <ADescriptionsItem :label="$gettext('Key Type')">
            {{ ingress.spec.tls.key_type || '2048' }}
          </ADescriptionsItem>
        </template>
        <ADescriptionsItem
          v-if="ingress.spec?.tls?.mode !== 'none'"
          :label="$gettext('Redirect to HTTPS')"
        >
          {{ ingress.spec?.tls?.redirect_https ? $gettext('Yes') : $gettext('No') }}
        </ADescriptionsItem>
        <ADescriptionsItem
          v-if="ingress.spec?.tls?.mode !== 'none'"
          :label="$gettext('HSTS')"
        >
          {{ ingress.spec?.tls?.hsts ? $gettext('Yes') : $gettext('No') }}
        </ADescriptionsItem>
      </ADescriptions>
      <AAlert
        v-if="cert?.state === 'pending'"
        type="info"
        show-icon
        class="mb-6 -mt-4"
        :title="$gettext('Only the HTTP listener is served until the certificate is issued.')"
      />

      <h3 class="mt-0 mb-2">
        {{ $gettext('Backends') }}
      </h3>
      <div
        v-for="backend in ingress.spec?.backends ?? []"
        :key="backend.name"
        class="mb-4"
        data-testid="ingress-backend"
      >
        <AFlex
          wrap
          align="center"
          gap="small"
          class="mb-2"
        >
          <span class="font-mono font-medium">{{ backend.name }}</span>
          <ATag class="me-0">
            {{ lbLabel(backend.lb) }}
          </ATag>
          <ATag v-if="backend.keepalive" class="me-0">
            keepalive {{ backend.keepalive }}
          </ATag>
        </AFlex>
        <ATable
          :columns="serverColumns"
          :data-source="backend.servers ?? []"
          :pagination="false"
          :scroll="{ x: 'max-content' }"
          row-key="address"
          size="small"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'address'">
              <AFlex wrap align="center" gap="small">
                <span
                  class="font-mono break-all"
                  :class="{ 'line-through opacity-60': record.down }"
                >{{ record.address }}</span>
                <ATag v-if="record.backup" color="orange" class="me-0">
                  backup
                </ATag>
                <ATag v-if="record.down" class="me-0">
                  down
                </ATag>
              </AFlex>
            </template>
            <template v-else-if="column.key === 'health'">
              <ABadge
                :status="healthBadge(backend, record).status"
                :text="healthBadge(backend, record).text"
              />
            </template>
            <template v-else-if="column.key === 'weight'">
              {{ record.weight || 1 }}
            </template>
            <template v-else-if="column.key === 'max_fails'">
              {{ record.max_fails ?? '-' }}
            </template>
            <template v-else-if="column.key === 'fail_timeout'">
              {{ record.fail_timeout || '-' }}
            </template>
          </template>
        </ATable>
      </div>

      <h3 class="mt-6 mb-2">
        {{ $gettext('Routes') }}
      </h3>
      <ATable
        :columns="routeColumns"
        :data-source="ingress.spec?.routes ?? []"
        :pagination="false"
        :scroll="{ x: 'max-content' }"
        row-key="path_prefix"
        size="small"
        class="mb-6"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'path_prefix'">
            <span class="font-mono break-all">{{ record.path_prefix }}</span>
          </template>
          <template v-else-if="column.key === 'backend'">
            <span class="font-mono">{{ record.backend }}</span>
          </template>
          <template v-else-if="column.key === 'options'">
            <AFlex wrap gap="small">
              <ATag v-if="record.websocket" color="purple" class="me-0">
                WebSocket
              </ATag>
              <ATag v-if="record.read_timeout" class="me-0">
                read {{ record.read_timeout }}
              </ATag>
              <ATag v-if="record.send_timeout" class="me-0">
                send {{ record.send_timeout }}
              </ATag>
              <ATag v-if="record.client_max_body_size" class="me-0">
                body {{ record.client_max_body_size }}
              </ATag>
              <span
                v-if="!record.websocket && !record.read_timeout && !record.send_timeout && !record.client_max_body_size"
                class="text-gray-500 dark:text-gray-400"
              >-</span>
            </AFlex>
          </template>
          <template v-else-if="column.key === 'allow_cidrs'">
            <AFlex
              v-if="record.allow_cidrs?.length"
              wrap
              gap="small"
            >
              <ATag
                v-for="cidr in record.allow_cidrs"
                :key="cidr"
                class="me-0 font-mono"
              >
                {{ cidr }}
              </ATag>
            </AFlex>
            <span v-else class="text-gray-500 dark:text-gray-400">{{ $gettext('Policy default') }}</span>
          </template>
        </template>
      </ATable>

      <h3 class="mt-0 mb-2">
        {{ $gettext('Policy') }}
      </h3>
      <ADescriptions
        bordered
        size="small"
        :column="1"
        class="mb-6"
      >
        <ADescriptionsItem :label="$gettext('Allowed Sources')">
          <AFlex
            v-if="ingress.spec?.policy?.allow_cidrs?.length"
            wrap
            gap="small"
          >
            <ATag
              v-for="cidr in ingress.spec.policy.allow_cidrs"
              :key="cidr"
              class="me-0 font-mono"
            >
              {{ cidr }}
            </ATag>
          </AFlex>
          <span v-else>{{ $gettext('Everyone') }}</span>
        </ADescriptionsItem>
        <ADescriptionsItem :label="$gettext('Rate Limit')">
          <template v-if="ingress.spec?.policy?.rate_limit">
            {{ $gettext('%{rps} requests/s per client, burst %{burst}', {
              rps: String(ingress.spec.policy.rate_limit.rps),
              burst: String(ingress.spec.policy.rate_limit.burst),
            }) }}
          </template>
          <span v-else>{{ $gettext('Unlimited') }}</span>
        </ADescriptionsItem>
        <ADescriptionsItem :label="$gettext('Max Body Size')">
          <span class="font-mono">{{ ingress.spec?.policy?.client_max_body_size || '-' }}</span>
        </ADescriptionsItem>
        <ADescriptionsItem :label="$gettext('Extra Headers')">
          <div
            v-if="ingress.spec?.policy?.extra_headers?.length"
            class="flex flex-col gap-1"
          >
            <span
              v-for="header in ingress.spec.policy.extra_headers"
              :key="header.name"
              class="font-mono break-all"
            >{{ header.name }}: {{ header.value }}</span>
          </div>
          <span v-else>-</span>
        </ADescriptionsItem>
      </ADescriptions>

      <h3 class="mt-0 mb-2">
        {{ $gettext('Conditions') }}
      </h3>
      <ATable
        :columns="conditionColumns"
        :data-source="ingress.status?.conditions ?? []"
        :pagination="false"
        :scroll="{ x: 'max-content' }"
        row-key="type"
        size="small"
        class="mb-6"
        data-testid="ingress-conditions"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'type'">
            {{ conditionLabel(record.type) }}
          </template>
          <template v-else-if="column.key === 'status'">
            <div class="flex flex-col">
              <ABadge
                :status="record.status ? 'success' : 'error'"
                :text="record.status ? $gettext('True') : $gettext('False')"
              />
              <span
                v-if="record.reason"
                class="font-mono text-xs text-gray-500 dark:text-gray-400"
              >{{ record.reason }}</span>
            </div>
          </template>
          <template v-else-if="column.key === 'message'">
            <!-- The table scrolls sideways on small screens, so the long nginx -t
                 output wraps inside a bounded cell instead of widening it. -->
            <div class="min-w-48 max-w-72 whitespace-pre-wrap break-all text-xs">
              {{ record.message || '-' }}
            </div>
          </template>
          <template v-else-if="column.key === 'last_transition_time'">
            {{ record.last_transition_time ? formatDateTime(record.last_transition_time) : '-' }}
          </template>
        </template>
      </ATable>

      <ACollapse
        v-model:active-key="activePanels"
        :items="[{ key: 'config', label: $gettext('Generated Site Config') }]"
      >
        <template #contentRender>
          <ASpin :spinning="isConfigLoading">
            <AAlert
              v-if="configError"
              type="warning"
              show-icon
              :title="configError"
            />
            <template v-else>
              <CodeEditor
                :content="generatedConfig"
                readonly
                default-height="360px"
              />
              <div v-if="siteLink" class="mt-2">
                <RouterLink :to="siteLink">
                  {{ $gettext('Open in site editor') }}
                </RouterLink>
              </div>
            </template>
          </ASpin>
        </template>
      </ACollapse>
    </template>
  </ADrawer>
</template>
