<script setup lang="ts">
import type { UploadProps } from 'antdv-next'
import type { PluginInfo, PluginInspect } from '@/api/plugin'
import type { PluginNodeResult } from '@/api/plugin_sync'
import { InboxOutlined } from '@antdv-next/icons'
import nodeApi from '@/api/node'
import pluginApi, { localizedPluginDescription, localizedPluginName } from '@/api/plugin'
import { setSyncPolicy, syncPlugin } from '@/api/plugin_sync'
import NodeSelector from '@/components/NodeSelector'
import gettext from '@/gettext'
import { getErrorMessage, resolveErrorMessage } from '@/lib/http'
import { capabilityLabel } from './capabilities'
import { useInstalledPlugin } from './inventory'
import { isCommunityTrust, isUnsignedTrust, packageTrustPreset } from './marketplace/trust'
import TrustDowngradeAlert from './marketplace/TrustDowngradeAlert.vue'
import PermissionList from './PermissionList.vue'

const emit = defineEmits<{
  installed: []
}>()

const open = defineModel<boolean>('open', { default: false })

/** Backend code for an unreadable package, also sent for an expired or unknown upload id. */
const PACKAGE_INVALID_CODE = 55007

const { message } = App.useApp()

const file = ref<File>()
const inspect = ref<PluginInspect>()
const inspecting = ref(false)
const installing = ref(false)
const enableAfterInstall = ref(true)
const error = ref('')

/** How far the bundle travels: this node only, every node, or a selection. */
type NodeTarget = 'none' | 'all' | 'selected'

const hasNodes = ref(false)
const nodeTarget = ref<NodeTarget>('none')
const targetNodeIds = ref<number[]>([])
const keepInSync = ref(false)
const syncResults = ref<PluginNodeResult[]>([])

const nodeTargetOptions = computed(() => [
  { value: 'none', label: $gettext('This node only') },
  { value: 'all', label: $gettext('All child nodes') },
  { value: 'selected', label: $gettext('Selected nodes') },
])

async function loadNodes() {
  try {
    const { data } = await nodeApi.getList({ enabled: true })
    hasNodes.value = data.length > 0
  }
  catch {
    hasNodes.value = false
  }
}

const manifest = computed(() => inspect.value?.manifest)
const manifestName = computed(() => (manifest.value ? localizedPluginName(manifest.value, gettext.current) : ''))
const manifestDescription = computed(() => (manifest.value ? localizedPluginDescription(manifest.value, gettext.current) : ''))
const permissions = computed(() => inspect.value?.permissions ?? [])
const requiresMissing = computed(() => inspect.value?.requires_missing ?? [])
const platforms = computed(() => inspect.value?.platforms ?? [])
// An older node does not report the field, so only an explicit false blocks.
const platformUnsupported = computed(() => inspect.value?.platform_supported === false)
// An older node reports no trust level, and then nothing is shown.
const trust = computed(() => packageTrustPreset(inspect.value?.trust))
const trustWarning = computed(() => {
  const level = inspect.value?.trust
  if (isUnsignedTrust(level))
    return $gettext('This plugin is not signed')
  if (level && isCommunityTrust(level))
    return $gettext('This is a community plugin')
  return ''
})
// Only an upgrade has an installed version to compare the trust with.
const installedPlugin = useInstalledPlugin(() => (inspect.value?.installed_version ? manifest.value?.id : undefined))

function reset() {
  file.value = undefined
  inspect.value = undefined
  error.value = ''
  enableAfterInstall.value = true
  nodeTarget.value = 'none'
  targetNodeIds.value = []
  keepInSync.value = false
  syncResults.value = []
}

// A selection that ends up empty would mean "every node" to the backend, so an
// explicit selection has to carry at least one node.
const canInstall = computed(() => Boolean(file.value)
  && requiresMissing.value.length === 0
  && !platformUnsupported.value
  && !inspecting.value
  && (nodeTarget.value !== 'selected' || targetNodeIds.value.length > 0))

/** Pushes the freshly installed plugin to the nodes the user picked. */
async function applyClusterOptions(pluginId: string) {
  const nodeIds = nodeTarget.value === 'selected' ? targetNodeIds.value : []

  if (keepInSync.value) {
    await setSyncPolicy(pluginId, {
      sync_policy: 'auto',
      sync_node_ids: nodeIds,
      sync_settings: false,
    })
  }

  if (nodeTarget.value === 'none')
    return

  const { results } = await syncPlugin(pluginId, nodeIds)
  syncResults.value = results
}

// The upload component is only a file picker here, so the request is cancelled
// and the bundle is inspected before anything is written to disk.
const beforeUpload: UploadProps['beforeUpload'] = async rawFile => {
  file.value = rawFile as File
  inspect.value = undefined
  error.value = ''
  inspecting.value = true

  try {
    inspect.value = await pluginApi.inspect(rawFile as File)
  }
  catch (e) {
    error.value = getErrorMessage(e, $gettext('Failed to read the plugin bundle'))
  }
  finally {
    inspecting.value = false
  }

  return false
}

function isPackageInvalid(e: unknown): boolean {
  const data = (e as { response?: { data?: { code?: unknown } } } | undefined)?.response?.data
  return Number(data?.code) === PACKAGE_INVALID_CODE
}

/**
 * Installs from the upload the inspect kept, so the package is not sent twice.
 * The node answers PACKAGE_INVALID_CODE when that upload expired or when it
 * does not know upload ids, and then the file is sent once instead.
 */
async function installPackage(packageFile: File, enable: boolean): Promise<PluginInfo> {
  const uploadId = inspect.value?.upload_id
  if (!uploadId)
    return pluginApi.install({ file: packageFile }, enable)

  try {
    // Quiet, so an expired upload does not flash an error before the fallback.
    return await pluginApi.install({ uploadId }, enable, { skipErrHandling: true })
  }
  catch (e) {
    if (!isPackageInvalid(e)) {
      // Show the toast the global handler skipped for this request.
      const text = await resolveErrorMessage(e, $gettext('Failed to install the plugin'))
      if (text)
        message.error(text)
      throw e
    }
  }

  return pluginApi.install({ file: packageFile }, enable)
}

async function install() {
  if (!file.value)
    return

  installing.value = true
  syncResults.value = []
  try {
    const info = await installPackage(file.value, enableAfterInstall.value)
    message.success($gettext('Plugin installed'))
    emit('installed')

    try {
      await applyClusterOptions(info.id)
    }
    catch (e) {
      // The local install already succeeded, so only the cluster step failed.
      error.value = getErrorMessage(e, $gettext('The plugin was installed here but could not be pushed to the nodes'))
      return
    }

    if (syncResults.value.some(result => !result.success)) {
      message.warning($gettext('Some nodes could not be synchronized'))
      return
    }

    open.value = false
    reset()
  }
  catch (e) {
    error.value = await resolveErrorMessage(e, $gettext('Failed to install the plugin'))
  }
  finally {
    installing.value = false
  }
}

watch(open, value => {
  if (value) {
    loadNodes()
    return
  }
  reset()
})
</script>

<template>
  <AModal
    v-model:open="open"
    :title="$gettext('Install plugin')"
    :width="640"
    :ok-text="$gettext('Install')"
    :cancel-text="$gettext('Cancel')"
    :ok-button-props="{ disabled: !canInstall }"
    :confirm-loading="installing"
    @ok="install"
  >
    <AUploadDragger
      :max-count="1"
      :show-upload-list="false"
      accept=".gz,.tgz,.tar.gz,application/gzip"
      :before-upload="beforeUpload"
    >
      <p class="text-3xl">
        <InboxOutlined />
      </p>
      <p class="mb-1 font-medium">
        {{ $gettext('Click or drag a plugin bundle here') }}
      </p>
      <p class="mb-0 text-gray-500">
        {{ file?.name || $gettext('A .tar.gz produced by the plugin author') }}
      </p>
    </AUploadDragger>

    <ASpin :spinning="inspecting">
      <AAlert
        v-if="error"
        type="error"
        show-icon
        class="mt-4"
        :title="error"
      />

      <template v-if="manifest">
        <ADescriptions
          class="mt-4"
          :column="1"
          size="small"
          bordered
        >
          <ADescriptionsItem :label="$gettext('Name')">
            <div class="flex flex-wrap items-center gap-2">
              <span>{{ manifestName }}</span>
              <ATooltip v-if="trust" :title="trust.hint()">
                <ATag :color="trust.color" class="m-0">
                  {{ trust.label() }}
                </ATag>
              </ATooltip>
            </div>
          </ADescriptionsItem>
          <ADescriptionsItem :label="$gettext('ID')">
            <span class="font-mono text-xs">{{ manifest.id }}</span>
          </ADescriptionsItem>
          <ADescriptionsItem :label="$gettext('Version')">
            {{ manifest.version }}
            <span v-if="inspect?.installed_version" class="ml-2 text-gray-500">
              {{ $gettext('(replaces %{version})', { version: inspect.installed_version }) }}
            </span>
          </ADescriptionsItem>
          <ADescriptionsItem v-if="manifestDescription" :label="$gettext('Description')">
            {{ manifestDescription }}
          </ADescriptionsItem>
          <ADescriptionsItem :label="$gettext('Capabilities')">
            <div v-if="manifest.capabilities?.length" class="flex flex-wrap gap-1">
              <ATag v-for="capability in manifest.capabilities" :key="capability">
                {{ capabilityLabel(capability) }}
              </ATag>
            </div>
            <span v-else class="text-gray-400">{{ $gettext('None') }}</span>
          </ADescriptionsItem>
          <ADescriptionsItem v-if="platforms.length > 0" :label="$gettext('Platforms')">
            <div class="flex flex-wrap gap-1">
              <ATag
                v-for="platform in platforms"
                :key="platform"
                :color="platform === inspect?.host_platform ? 'blue' : undefined"
              >
                {{ platform === 'any' ? $gettext('Any') : platform }}
              </ATag>
            </div>
          </ADescriptionsItem>
        </ADescriptions>

        <AAlert
          v-if="platformUnsupported"
          type="error"
          show-icon
          class="mt-4"
          :title="$gettext('This package has no build for this node (%{platform})', { platform: inspect?.host_platform ?? '' })"
          :description="$gettext('Download the package built for this platform, or the portable package, and try again.')"
        />

        <AAlert
          v-if="requiresMissing.length > 0"
          type="error"
          show-icon
          class="mt-4"
          :title="$gettext('Install the plugins below first')"
        >
          <template #description>
            <ul class="mb-0 pl-4">
              <li v-for="requirement in requiresMissing" :key="requirement.id">
                <span class="font-mono text-xs">{{ requirement.id }}</span>
                <span v-if="requirement.version"> {{ requirement.version }}</span>
              </li>
            </ul>
          </template>
        </AAlert>

        <TrustDowngradeAlert
          class="mt-4"
          :next="inspect?.trust"
          :installed="installedPlugin?.trust"
        />

        <AAlert
          v-if="trust && trustWarning"
          type="warning"
          show-icon
          class="mt-4"
          :title="trustWarning"
          :description="trust.hint()"
        />

        <AAlert
          v-if="inspect?.permissions_changed"
          type="warning"
          show-icon
          class="mt-4"
          :title="$gettext('This version asks for permissions you have not approved yet.')"
          :description="$gettext('Review the list below before installing. The plugin stays disabled until you approve them.')"
        />

        <div class="mt-4">
          <h4 class="mb-2">
            {{ $gettext('Requested permissions') }}
          </h4>
          <PermissionList :permissions="permissions" />
        </div>

        <ACheckbox v-model:checked="enableAfterInstall" class="mt-4">
          {{ $gettext('Enable after install') }}
        </ACheckbox>

        <template v-if="hasNodes">
          <ADivider class="my-4" />

          <h4 class="mb-2">
            {{ $gettext('Also install to child nodes') }}
          </h4>
          <ARadioGroup
            v-model:value="nodeTarget"
            option-type="button"
            button-style="solid"
            size="small"
            :options="nodeTargetOptions"
          />

          <div v-if="nodeTarget === 'selected'" class="mt-3">
            <NodeSelector v-model:target="targetNodeIds" hidden-local />
          </div>

          <ACheckbox v-model:checked="keepInSync" class="mt-3">
            {{ $gettext('Keep in sync automatically') }}
          </ACheckbox>
          <p class="mb-0 mt-1 text-gray-500">
            {{ $gettext('The version, the enabled state and future updates are kept aligned on those nodes.') }}
          </p>

          <div v-if="syncResults.length > 0" class="sync-results mt-4">
            <div
              v-for="result in syncResults"
              :key="result.node_id"
              class="sync-result-row"
            >
              <span class="font-medium">{{ result.node }}</span>
              <ATag v-if="result.success" color="green">
                {{ result.actions.length > 0 ? result.actions.join(', ') : $gettext('Already in sync') }}
              </ATag>
              <ATooltip v-else :title="result.error">
                <ATag color="red">
                  {{ $gettext('Failed') }}
                </ATag>
              </ATooltip>
            </div>
          </div>
        </template>
      </template>
    </ASpin>
  </AModal>
</template>

<style lang="less" scoped>
.sync-results {
  border: 1px solid var(--ant-color-border-secondary);
  border-radius: var(--ant-border-radius);
  overflow: hidden;
}

.sync-result-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 12px;

  & + & {
    border-top: 1px solid var(--ant-color-border-secondary);
  }
}
</style>
