<script setup lang="ts">
import type { UploadProps } from 'antdv-next'
import type { PluginInspect } from '@/api/plugin'
import { InboxOutlined } from '@antdv-next/icons'
import pluginApi from '@/api/plugin'
import { getErrorMessage } from '@/lib/http'
import PermissionList from './PermissionList.vue'

const emit = defineEmits<{
  installed: []
}>()

const open = defineModel<boolean>('open', { default: false })

const { message } = App.useApp()

const file = ref<File>()
const inspect = ref<PluginInspect>()
const inspecting = ref(false)
const installing = ref(false)
const enableAfterInstall = ref(true)
const error = ref('')

const manifest = computed(() => inspect.value?.manifest)
const permissions = computed(() => inspect.value?.permissions ?? [])
const requiresMissing = computed(() => inspect.value?.requires_missing ?? [])
const canInstall = computed(() => Boolean(file.value) && requiresMissing.value.length === 0 && !inspecting.value)

function reset() {
  file.value = undefined
  inspect.value = undefined
  error.value = ''
  enableAfterInstall.value = true
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

async function install() {
  if (!file.value)
    return

  installing.value = true
  try {
    await pluginApi.install(file.value, enableAfterInstall.value)
    message.success($gettext('Plugin installed'))
    open.value = false
    reset()
    emit('installed')
  }
  catch (e) {
    error.value = getErrorMessage(e, $gettext('Failed to install the plugin'))
  }
  finally {
    installing.value = false
  }
}

watch(open, value => {
  if (!value)
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
            {{ manifest.name }}
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
          <ADescriptionsItem v-if="manifest.description" :label="$gettext('Description')">
            {{ manifest.description }}
          </ADescriptionsItem>
          <ADescriptionsItem :label="$gettext('Capabilities')">
            <div v-if="manifest.capabilities?.length" class="flex flex-wrap gap-1">
              <ATag v-for="capability in manifest.capabilities" :key="capability">
                {{ capability }}
              </ATag>
            </div>
            <span v-else class="text-gray-400">{{ $gettext('None') }}</span>
          </ADescriptionsItem>
        </ADescriptions>

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
      </template>
    </ASpin>
  </AModal>
</template>
