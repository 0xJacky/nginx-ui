<script setup lang="ts">
import type { CatalogEntry, CatalogRelease } from '@/api/plugin_marketplace'
import { GlobalOutlined, LinkOutlined } from '@antdv-next/icons'
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
import PluginIcon from '../PluginIcon.vue'
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

const trustTone: Record<string, string> = { blue: 'is-accent', green: 'is-success', orange: 'is-warning' }

const facts = computed(() => {
  const entry = current.value
  if (!entry)
    return []
  const items = [
    { key: 'id', label: $gettext('ID'), value: entry.id, mono: true },
    ...(entry.author ? [{ key: 'author', label: $gettext('Author'), value: entry.author, mono: false }] : []),
    ...(entry.license ? [{ key: 'license', label: $gettext('License'), value: entry.license, mono: false }] : []),
    ...(entry.installed_version
      ? [{ key: 'installed', label: $gettext('Installed version'), value: `v${entry.installed_version}`, mono: false }]
      : []),
  ]
  return items
})

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
        <div class="detail">
          <div class="detail-head">
            <PluginIcon :src="current.icon_url" :size="48" />
            <div class="min-w-0 flex-1">
              <div class="pill-row mb-2">
                <ATooltip :title="trust.hint()">
                  <span class="pill" :class="trustTone[trust.color] ?? 'is-muted'">{{ trust.label() }}</span>
                </ATooltip>
                <span
                  v-for="capability in current.capabilities ?? []"
                  :key="capability"
                  class="pill is-accent"
                >
                  {{ capability }}
                </span>
                <span v-if="current.stage && current.stage !== 'production'" class="pill is-purple">
                  {{ current.stage }}
                </span>
              </div>
              <p class="overview-description">
                {{ description || $gettext('No description provided.') }}
              </p>
            </div>
          </div>

          <div class="fact-grid">
            <div
              v-for="fact in facts"
              :key="fact.key"
              class="fact"
              :class="{ 'is-wide': fact.mono }"
            >
              <span class="fact-label">{{ fact.label }}</span>
              <span class="fact-value" :class="{ 'is-mono': fact.mono }">{{ fact.value }}</span>
            </div>
          </div>

          <dl class="detail-list">
            <div class="detail-row">
              <dt>{{ $gettext('Source') }}</dt>
              <dd class="detail-source">
                {{ current.source }}
              </dd>
            </div>
            <div v-if="links.length > 0" class="detail-row">
              <dt>{{ $gettext('Links') }}</dt>
              <dd class="pill-row">
                <a
                  v-for="link in links"
                  :key="link.key"
                  :href="link.url"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="pill is-link"
                >
                  <component :is="link.icon" />
                  {{ link.label }}
                </a>
              </dd>
            </div>
          </dl>

          <section class="overview-section">
            <div class="section-head">
              <h4 class="section-title">
                {{ $gettext('Requested permissions') }}
              </h4>
              <span v-if="permissions.length" class="section-count">{{ permissions.length }}</span>
            </div>
            <PermissionList :permissions="permissions" />
          </section>

          <section class="overview-section">
            <div class="section-head">
              <h4 class="section-title">
                {{ $gettext('Versions') }}
              </h4>
              <span v-if="hostPlatform" class="section-hint">
                {{ $gettext('This node runs %{platform}, its build is highlighted.', { platform: hostPlatform }) }}
              </span>
            </div>
            <ul class="release-list">
              <li
                v-for="release in current.releases ?? []"
                :key="release.version"
                class="release"
                :class="{
                  'is-latest': release.version === current.installable_release?.version,
                  'is-yanked': release.yanked,
                }"
              >
                <div class="release-main">
                  <span class="release-version">v{{ release.version }}</span>
                  <span v-if="release.yanked" class="pill is-danger">{{ $gettext('Yanked') }}</span>
                  <span v-else-if="release.version === current.installable_release?.version" class="pill is-accent">{{ $gettext('Latest') }}</span>
                  <span v-if="release.released_at" class="release-date">{{ release.released_at.slice(0, 10) }}</span>
                </div>
                <div class="pill-row">
                  <template v-if="releasePlatforms(release).length">
                    <span
                      v-for="platform in releasePlatforms(release)"
                      :key="platform"
                      class="pill"
                      :class="{ 'is-accent': isHostBuild(release, platform) }"
                    >
                      {{ platformLabel(platform) }}
                    </span>
                  </template>
                  <span v-else class="pill">{{ $gettext('Any') }}</span>
                </div>
              </li>
            </ul>
          </section>

          <section v-if="renderedReadme" class="overview-section">
            <div class="section-head">
              <h4 class="section-title">
                {{ $gettext('Readme') }}
              </h4>
            </div>
            <div v-dompurify-html="renderedReadme" class="plugin-readme" />
          </section>
        </div>
      </template>
    </ASpin>
  </ADrawer>
</template>

<style lang="less" scoped>
@import '../plugin-detail.less';

.detail {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.fact-value.is-mono {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 13px;
}

.detail-source {
  font-size: 12px;
  color: var(--ant-color-text-secondary);
  overflow-wrap: anywhere;
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
