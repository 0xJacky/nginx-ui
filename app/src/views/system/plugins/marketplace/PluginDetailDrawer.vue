<script setup lang="ts">
import type { CatalogEntry, CatalogRelease } from '@/api/plugin_marketplace'
import { AppstoreOutlined, GlobalOutlined, LinkOutlined } from '@antdv-next/icons'
import { useWindowSize } from '@vueuse/core'
import { marked } from 'marked'
import {
  ANY_PLATFORM,
  catalogEntryDescription,
  catalogEntryName,
  getMarketplacePlugin,
  releasePlatforms,
} from '@/api/plugin_marketplace'
import gettext from '@/gettext'
import { getErrorMessage } from '@/lib/http'
import PermissionList from '../PermissionList.vue'
import { trustPreset } from './trust'

const props = defineProps<{
  entry?: CatalogEntry
}>()

const emit = defineEmits<{
  install: [entry: CatalogEntry]
}>()

const open = defineModel<boolean>('open', { default: false })

const { width: windowWidth } = useWindowSize()
// A phone gets the whole screen instead of an unusable sliver.
const drawerSize = computed(() => Math.min(720, windowWidth.value))

const loading = ref(false)
const error = ref('')
const readme = ref('')
const detail = ref<CatalogEntry>()
const hostPlatform = ref('')

const current = computed(() => detail.value ?? props.entry)
const name = computed(() => (current.value ? catalogEntryName(current.value, gettext.current) : ''))
const description = computed(() => (current.value ? catalogEntryDescription(current.value, gettext.current) : ''))
const trust = computed(() => trustPreset(current.value?.trust))
const permissions = computed(() => current.value?.installable_release?.manifest?.permissions ?? [])
const renderedReadme = computed(() => (readme.value ? marked.parse(readme.value) as string : ''))

const canInstall = computed(() => Boolean(current.value?.installable_release)
  && (!current.value?.installed_version || current.value?.update_available))

const links = computed(() => [
  { key: 'homepage', icon: GlobalOutlined, label: $gettext('Homepage'), url: current.value?.homepage_url },
  { key: 'repository', icon: LinkOutlined, label: $gettext('Repository'), url: current.value?.repository_url },
  { key: 'release_notes', icon: LinkOutlined, label: $gettext('Release notes'), url: current.value?.installable_release?.release_notes_url },
].filter(link => Boolean(link.url)))

const releaseColumns = computed(() => [
  { title: $gettext('Version'), dataIndex: 'version', width: 140 },
  { title: $gettext('Released'), dataIndex: 'released_at', width: 140 },
  { title: $gettext('Platforms'), dataIndex: 'platforms' },
])

/** Whether a platform tag stands for the build this node would install. */
function isHostBuild(release: CatalogRelease, platform: string) {
  if (!hostPlatform.value)
    return false
  if (platform === hostPlatform.value)
    return true
  // "any" only serves this node when no dedicated build exists.
  return platform === ANY_PLATFORM && !releasePlatforms(release).includes(hostPlatform.value)
}

function platformLabel(platform: string) {
  return platform === ANY_PLATFORM ? $gettext('Any') : platform
}

async function load() {
  const id = props.entry?.id
  if (!id)
    return

  loading.value = true
  error.value = ''
  readme.value = ''
  detail.value = undefined
  try {
    const response = await getMarketplacePlugin(id, props.entry?.source)
    detail.value = response.plugin
    readme.value = response.readme
    hostPlatform.value = response.host_platform ?? ''
  }
  catch (e) {
    error.value = getErrorMessage(e, $gettext('Failed to load the plugin details'))
  }
  finally {
    loading.value = false
  }
}

watch(open, value => {
  if (value)
    load()
})
</script>

<template>
  <ADrawer
    v-model:open="open"
    :title="name"
    :size="drawerSize"
    placement="right"
  >
    <template #extra>
      <AButton
        type="primary"
        :disabled="!canInstall"
        @click="current && emit('install', current)"
      >
        {{ current?.update_available ? $gettext('Update') : $gettext('Install') }}
      </AButton>
    </template>

    <ASpin :spinning="loading">
      <AAlert
        v-if="error"
        type="error"
        show-icon
        class="mb-4"
        :title="error"
      />

      <template v-if="current">
        <div class="mb-4 flex items-start gap-3">
          <img
            v-if="current.icon_url"
            :src="current.icon_url"
            class="detail-icon"
            alt=""
          >
          <AppstoreOutlined v-else class="detail-icon-fallback" />
          <div class="min-w-0">
            <div class="mb-1 flex flex-wrap items-center gap-2">
              <ATooltip :title="trust.hint()">
                <ATag :color="trust.color" class="m-0">
                  {{ trust.label() }}
                </ATag>
              </ATooltip>
              <ATag v-for="capability in current.capabilities ?? []" :key="capability" class="m-0">
                {{ capability }}
              </ATag>
            </div>
            <p class="mb-0 text-gray-500">
              {{ description }}
            </p>
          </div>
        </div>

        <ADescriptions :column="1" size="small" bordered>
          <ADescriptionsItem :label="$gettext('ID')">
            <span class="font-mono text-xs">{{ current.id }}</span>
          </ADescriptionsItem>
          <ADescriptionsItem v-if="current.author" :label="$gettext('Author')">
            {{ current.author }}
          </ADescriptionsItem>
          <ADescriptionsItem v-if="current.license" :label="$gettext('License')">
            {{ current.license }}
          </ADescriptionsItem>
          <ADescriptionsItem v-if="current.installed_version" :label="$gettext('Installed version')">
            {{ current.installed_version }}
          </ADescriptionsItem>
          <ADescriptionsItem :label="$gettext('Source')">
            <span class="break-all text-xs text-gray-500">{{ current.source }}</span>
          </ADescriptionsItem>
        </ADescriptions>

        <div v-if="links.length > 0" class="mt-4 flex flex-wrap gap-4">
          <a
            v-for="link in links"
            :key="link.key"
            :href="link.url"
            target="_blank"
            rel="noopener noreferrer"
          >
            <component :is="link.icon" />
            {{ link.label }}
          </a>
        </div>

        <h3 class="mt-6">
          {{ $gettext('Requested permissions') }}
        </h3>
        <PermissionList :permissions="permissions" />

        <h3 class="mt-6">
          {{ $gettext('Versions') }}
        </h3>
        <p v-if="hostPlatform" class="mb-2 text-xs text-gray-500">
          {{ $gettext('This node runs %{platform}, its build is highlighted.', { platform: hostPlatform }) }}
        </p>
        <ATable
          :columns="releaseColumns"
          :data-source="current.releases ?? []"
          row-key="version"
          size="small"
          :pagination="false"
          :scroll="{ x: 420 }"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.dataIndex === 'version'">
              <span>{{ record.version }}</span>
              <ATag v-if="record.yanked" color="red" class="ml-2">
                {{ $gettext('Yanked') }}
              </ATag>
              <ATag v-else-if="record.version === current?.installable_release?.version" color="blue" class="ml-2">
                {{ $gettext('Latest') }}
              </ATag>
            </template>
            <template v-else-if="column.dataIndex === 'released_at'">
              <span class="text-xs text-gray-500">{{ record.released_at?.slice(0, 10) || '-' }}</span>
            </template>
            <template v-else-if="column.dataIndex === 'platforms'">
              <div v-if="releasePlatforms(record).length" class="flex flex-wrap gap-1">
                <ATag
                  v-for="platform in releasePlatforms(record)"
                  :key="platform"
                  :color="isHostBuild(record, platform) ? 'blue' : undefined"
                  class="m-0"
                >
                  {{ platformLabel(platform) }}
                </ATag>
              </div>
              <span v-else class="text-gray-400">{{ $gettext('Any') }}</span>
            </template>
          </template>
        </ATable>

        <template v-if="renderedReadme">
          <h3 class="mt-6">
            {{ $gettext('Readme') }}
          </h3>
          <div v-dompurify-html="renderedReadme" class="plugin-readme" />
        </template>
      </template>
    </ASpin>
  </ADrawer>
</template>

<style lang="less" scoped>
.detail-icon,
.detail-icon-fallback {
  width: 48px;
  height: 48px;
  flex: none;
  object-fit: contain;
  border-radius: 10px;
}

.detail-icon-fallback {
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 24px;
  color: var(--ant-color-text-quaternary);
  background-color: var(--ant-color-fill-tertiary);
}

.plugin-readme {
  word-break: break-word;

  :deep(img) {
    max-width: 100%;
  }

  :deep(pre) {
    padding: 12px;
    overflow: auto;
    border-radius: 6px;
    background-color: var(--ant-color-fill-tertiary);
  }

  :deep(table) {
    width: 100%;
    border-collapse: collapse;
  }

  :deep(th),
  :deep(td) {
    padding: 6px 8px;
    border: 1px solid var(--ant-color-split);
  }
}
</style>
