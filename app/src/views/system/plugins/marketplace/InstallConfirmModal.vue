<script setup lang="ts">
import type { ProgressProps } from 'antdv-next'
import type { PluginInfo } from '@/api/plugin'
import type { CatalogEntry, CatalogRelease, PluginInstallProgress, PluginInstallStatus } from '@/api/plugin_marketplace'
import { useIntervalFn, useTimeoutFn } from '@vueuse/core'
import pluginApi from '@/api/plugin'
import {
  catalogEntryName,
  getMarketplacePlugin,
  installFromMarketplace,
  PLUGIN_INSTALL_PROGRESS_EVENT,
} from '@/api/plugin_marketplace'
import gettext from '@/gettext'
import { getErrorMessage } from '@/lib/http'
import { useWebSocketEventBusStore } from '@/pinia'
import { entryChannel, hasStableRelease, isDowngrade, releaseChannel } from '../channel'
import ChannelTag from '../ChannelTag.vue'
import { installReplacesText } from '../conflicts'
import { useInstalledPlugin } from '../inventory'
import { formatMemory, isBelowRecommended, memoryWarning, recommendedMemory, useSystemMemory } from '../memory'
import PermissionList from '../PermissionList.vue'
import { permissionReasons } from '../permissions'
import { usePackageConflicts } from '../useConflicts'
import { useSourceName } from './sources'
import { isCommunityTrust, trustPreset } from './trust'
import TrustDowngradeAlert from './TrustDowngradeAlert.vue'

const props = defineProps<{
  /** Entry to install. When absent it is fetched from pluginId. */
  entry?: CatalogEntry
  /** Used when the caller only knows the id, e.g. the DNS-01 shortcut. */
  pluginId?: string
  version?: string
  /** Starts the install as soon as the dialog opens. */
  autoStart?: boolean
}>()

const emit = defineEmits<{
  installed: [info: PluginInfo]
}>()

const open = defineModel<boolean>('open', { default: false })

const { message } = App.useApp()
const eventBus = useWebSocketEventBusStore()

const resolved = ref<CatalogEntry>()
const loading = ref(false)
const installing = ref(false)
const error = ref('')
const enableAfterInstall = ref(true)

const progress = ref(0)
const phase = ref<PluginInstallStatus>('downloading')
const activeId = ref('')
const activePlatform = ref('')
const receivedEvent = ref(false)
const polling = ref(false)

let subscriptionId = ''

const entry = computed(() => props.entry ?? resolved.value)
const targetId = computed(() => entry.value?.id ?? props.pluginId ?? '')
const sourceName = useSourceName()
const name = computed(() => (entry.value ? catalogEntryName(entry.value, gettext.current) : targetId.value))

const release = computed<CatalogRelease | undefined>(() => {
  const current = entry.value
  if (!current)
    return undefined
  if (props.version)
    return current.releases?.find(item => item.version === props.version) ?? current.installable_release
  return current.installable_release
})

const permissions = computed(() => release.value?.manifest?.permissions ?? [])
const reasons = computed(() => permissionReasons(release.value?.manifest, gettext.current))
const requires = computed(() => release.value?.manifest?.requires ?? [])
const trust = computed(() => trustPreset(entry.value?.trust))
// The release being installed decides, a plugin can ship a beta next to a stable one.
const channel = computed(() => (release.value ? releaseChannel(release.value) : entryChannel(entry.value)))
const isOlder = computed(() => isDowngrade(entry.value?.installed_version, release.value?.version))
const recommendedMb = computed(() => recommendedMemory(release.value?.manifest))
const systemMb = useSystemMemory()
const lowMemory = computed(() => isBelowRecommended(recommendedMb.value, systemMb.value))
// Turning the package on turns off the enabled plugins that conflict with it.
const conflictingNames = usePackageConflicts(
  () => (release.value?.manifest ? { id: targetId.value, conflicts: release.value.manifest.conflicts } : undefined),
  () => open.value && enableAfterInstall.value,
)
const showCommunityWarning = computed(() => Boolean(entry.value) && isCommunityTrust(entry.value?.trust))
const isUpgrade = computed(() => Boolean(entry.value?.installed_version))
// Looked up while the dialog is open, and only for an upgrade.
const installedPlugin = useInstalledPlugin(() => (open.value && isUpgrade.value ? entry.value?.id : undefined))
// A plugin that only ships test versions so far.
const noStableVersion = computed(() => Boolean(entry.value) && channel.value !== 'stable' && !hasStableRelease(entry.value))
const title = computed(() => {
  if (isOlder.value)
    return $gettext('Install a previous version')
  return isUpgrade.value ? $gettext('Update plugin') : $gettext('Install plugin')
})
const okText = computed(() => {
  if (isOlder.value)
    return $gettext('Install this version')
  return isUpgrade.value ? $gettext('Update') : $gettext('Install')
})

const phaseLabels: Record<PluginInstallStatus, () => string> = {
  downloading: () => $gettext('Downloading the package'),
  verifying: () => $gettext('Checking the package'),
  installing: () => $gettext('Installing'),
  done: () => $gettext('Done'),
  error: () => $gettext('Failed'),
}

const phaseLabel = computed(() => {
  let label = phaseLabels[phase.value]?.() ?? ''
  // The catalog serves one package per platform, name the one being fetched.
  if (activePlatform.value && phase.value !== 'done' && phase.value !== 'error')
    label = `${label} (${activePlatform.value})`
  if (activeId.value && activeId.value !== targetId.value)
    return $gettext('%{phase}: %{plugin}', { phase: label, plugin: activeId.value })
  return label
})

const progressStatus = computed<ProgressProps['status']>(() => {
  if (error.value)
    return 'exception'
  if (phase.value === 'done')
    return 'success'
  return 'active'
})

// The websocket may be unavailable behind some proxies, so a silent install
// falls back to watching the plugin list instead of freezing at 0%.
const { pause: stopPolling, resume: startPolling } = useIntervalFn(async () => {
  if (!installing.value)
    return
  try {
    const list = await pluginApi.getList()
    if (list.some(item => item.id === targetId.value))
      progress.value = Math.max(progress.value, 90)
  }
  catch {
    // The list is only a progress hint, a failed poll changes nothing.
  }
}, 3000, { immediate: false })

const { start: armFallback, stop: disarmFallback } = useTimeoutFn(() => {
  if (!installing.value || receivedEvent.value)
    return
  polling.value = true
  startPolling()
}, 10_000, { immediate: false })

function onProgress(data: PluginInstallProgress) {
  if (!installing.value)
    return

  receivedEvent.value = true
  activeId.value = data.plugin_id
  activePlatform.value = data.platform ?? ''
  phase.value = data.status
  if (data.status !== 'error')
    progress.value = Math.max(progress.value, Math.min(100, Math.round(data.progress)))
}

function subscribe() {
  if (subscriptionId)
    return
  subscriptionId = eventBus.subscribe<PluginInstallProgress>(PLUGIN_INSTALL_PROGRESS_EVENT, onProgress)
}

function unsubscribe() {
  if (!subscriptionId)
    return
  eventBus.unsubscribe(subscriptionId)
  subscriptionId = ''
}

function resetProgress() {
  progress.value = 0
  phase.value = 'downloading'
  activePlatform.value = ''
  activeId.value = ''
  receivedEvent.value = false
  polling.value = false
}

function reset() {
  resolved.value = undefined
  error.value = ''
  installing.value = false
  enableAfterInstall.value = true
  resetProgress()
  disarmFallback()
  stopPolling()
  unsubscribe()
}

async function loadEntry() {
  if (props.entry || !props.pluginId)
    return

  loading.value = true
  error.value = ''
  try {
    const detail = await getMarketplacePlugin(props.pluginId)
    resolved.value = detail.plugin
  }
  catch (e) {
    error.value = getErrorMessage(e, $gettext('Failed to load the plugin from the marketplace'))
  }
  finally {
    loading.value = false
  }
}

async function install() {
  if (!targetId.value || installing.value)
    return

  error.value = ''
  installing.value = true
  resetProgress()
  subscribe()
  armFallback()

  try {
    const info = await installFromMarketplace({
      id: targetId.value,
      version: props.version || release.value?.version,
      source: entry.value?.source,
      enable: enableAfterInstall.value,
      approve_permissions: true,
      replace_conflicts: conflictingNames.value.length > 0,
    })
    progress.value = 100
    phase.value = 'done'
    message.success($gettext('Plugin installed'))
    emit('installed', info)
    open.value = false
  }
  catch (e) {
    phase.value = 'error'
    error.value = getErrorMessage(e, $gettext('Failed to install the plugin'))
  }
  finally {
    installing.value = false
    disarmFallback()
    stopPolling()
    unsubscribe()
  }
}

watch(open, async value => {
  if (!value) {
    reset()
    return
  }

  reset()
  await loadEntry()
  if (props.autoStart && !error.value)
    void install()
})

onUnmounted(() => {
  disarmFallback()
  stopPolling()
  unsubscribe()
})
</script>

<template>
  <AModal
    v-model:open="open"
    :title="title"
    :width="600"
    :mask-closable="!installing"
    :closable="!installing"
    :ok-text="okText"
    :cancel-text="$gettext('Cancel')"
    :ok-button-props="{ disabled: !release || installing }"
    :confirm-loading="installing"
    @ok="install"
  >
    <ASpin :spinning="loading">
      <AAlert
        v-if="error"
        type="error"
        show-icon
        class="mb-4"
        :title="error"
      />

      <ADescriptions :column="1" size="small" bordered>
        <ADescriptionsItem :label="$gettext('Name')">
          <div class="flex flex-wrap items-center gap-2">
            <span>{{ name }}</span>
            <ATooltip :title="trust.hint()">
              <ATag :color="trust.color" class="m-0">
                {{ trust.label() }}
              </ATag>
            </ATooltip>
          </div>
        </ADescriptionsItem>
        <ADescriptionsItem :label="$gettext('ID')">
          <span class="font-mono text-xs">{{ targetId }}</span>
        </ADescriptionsItem>
        <ADescriptionsItem :label="$gettext('Version')">
          <template v-if="release">
            <span>{{ release.version }}</span>
            <span v-if="channel !== 'stable'" class="ml-2 inline-flex align-middle"><ChannelTag :channel="channel" /></span>
          </template>
          <span v-else class="text-gray-400">{{ $gettext('No release available for this platform') }}</span>
          <span v-if="entry?.installed_version" class="ml-2 text-gray-500">
            {{ $gettext('(replaces %{version})', { version: entry.installed_version }) }}
          </span>
        </ADescriptionsItem>
        <ADescriptionsItem v-if="recommendedMb > 0" :label="$gettext('Recommended memory')">
          {{ formatMemory(recommendedMb) }}
        </ADescriptionsItem>
        <ADescriptionsItem v-if="entry?.source" :label="$gettext('Source')">
          <span class="break-all text-xs text-gray-500" :title="entry.source">{{ sourceName(entry.source) }}</span>
        </ADescriptionsItem>
      </ADescriptions>

      <TrustDowngradeAlert
        class="mt-4"
        :next="entry?.trust"
        :installed="installedPlugin?.trust"
      />

      <AAlert
        v-if="isOlder"
        type="warning"
        show-icon
        class="mt-4"
        :title="$gettext('This version is older than the installed version')"
        :description="$gettext('Data created by the newer version may not be compatible with this version.')"
      />

      <AAlert
        v-if="noStableVersion"
        type="info"
        show-icon
        class="mt-4"
        :title="$gettext('No stable release of this plugin is available yet.')"
        :description="$gettext('Newer releases of this channel are received until a stable release is available.')"
      />

      <AAlert
        v-if="lowMemory"
        type="warning"
        show-icon
        class="mt-4"
        :title="memoryWarning(recommendedMb, systemMb)"
      />

      <AAlert
        v-if="conflictingNames.length > 0"
        type="warning"
        show-icon
        class="mt-4"
        :title="installReplacesText(conflictingNames)"
      />

      <AAlert
        v-if="showCommunityWarning"
        type="warning"
        show-icon
        class="mt-4"
        :title="$gettext('This is a community plugin')"
        :description="trust.hint()"
      />

      <AAlert
        v-if="requires.length > 0"
        type="info"
        show-icon
        class="mt-4"
        :title="$gettext('The plugins below are installed together with it')"
      >
        <template #description>
          <ul class="mb-0 pl-4">
            <li v-for="requirement in requires" :key="requirement.id">
              <span class="font-mono text-xs">{{ requirement.id }}</span>
              <span v-if="requirement.version"> {{ requirement.version }}</span>
            </li>
          </ul>
        </template>
      </AAlert>

      <div class="mt-4">
        <h4 class="mb-2">
          {{ $gettext('Requested permissions') }}
        </h4>
        <PermissionList :permissions="permissions" :reasons="reasons" />
      </div>

      <ACheckbox v-model:checked="enableAfterInstall" :disabled="installing" class="mt-4">
        {{ $gettext('Enable after install') }}
      </ACheckbox>

      <div v-if="installing || progress > 0" class="mt-4">
        <AProgress :percent="progress" :status="progressStatus" />
        <p class="mb-0 text-xs text-gray-500">
          {{ phaseLabel }}
          <span v-if="polling"> · {{ $gettext('Live progress is unavailable, checking the plugin list instead') }}</span>
        </p>
      </div>
    </ASpin>
  </AModal>
</template>
