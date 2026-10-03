<script setup lang="ts">
import type { CatalogEntry } from '@/api/plugin_marketplace'
import { GlobalOutlined, LinkOutlined } from '@antdv-next/icons'
import { useWindowSize } from '@vueuse/core'
import { marked } from 'marked'
import {
  catalogEntryDescription,
  catalogEntryName,
  catalogScreenshotCaption,
  catalogScreenshotURL,
  getMarketplacePlugin,
} from '@/api/plugin_marketplace'
import gettext from '@/gettext'
import { getErrorMessage } from '@/lib/http'
import { useSettingsStore } from '@/pinia'
import { capabilityLabel } from '../capabilities'
import { categoryLabel } from '../categories'
import { channelHint, channelLabel, compareVersions, entryChannel, installableReleases, releaseChannel } from '../channel'
import { useInstalledPlugin } from '../inventory'
import { formatMemory, isBelowRecommended, memoryWarning, recommendedMemory, useSystemMemory } from '../memory'
import PermissionList from '../PermissionList.vue'
import { permissionReasons } from '../permissions'
import PluginIcon from '../PluginIcon.vue'
import { useReplacePlugin } from '../replace'
import { useSourceIcon, useSourceName } from './sources'
import { findTrustedOffer, trustedOfferAction, trustPreset } from './trust'

const props = defineProps<{
  entry?: CatalogEntry
}>()

const emit = defineEmits<{
  install: [entry: CatalogEntry, version?: string]
}>()

const open = defineModel<boolean>('open', { default: false })

const { width: windowWidth } = useWindowSize()
// A phone gets the whole screen instead of an unusable sliver.
const drawerSize = computed(() => Math.min(720, windowWidth.value))

const loading = ref(false)
const error = ref('')
const readme = ref('')
const detail = ref<CatalogEntry>()

const current = computed(() => detail.value ?? props.entry)
const sourceName = useSourceName()
const sourceIcon = useSourceIcon()
const name = computed(() => (current.value ? catalogEntryName(current.value, gettext.current) : ''))
const description = computed(() => (current.value ? catalogEntryDescription(current.value, gettext.current) : ''))
const trust = computed(() => trustPreset(current.value?.trust))
const permissions = computed(() => current.value?.installable_release?.manifest?.permissions ?? [])
const reasons = computed(() => permissionReasons(current.value?.installable_release?.manifest, gettext.current))
const recommendedMb = computed(() => recommendedMemory(current.value?.installable_release?.manifest))
const systemMb = useSystemMemory()
const lowMemory = computed(() => isBelowRecommended(recommendedMb.value, systemMb.value))
const channel = computed(() => entryChannel(current.value))
const installed = useInstalledPlugin(() => current.value?.id)
const offer = computed(() => findTrustedOffer(installed.value?.trust, current.value))
const { replacingId, confirmReplace } = useReplacePlugin()
const settings = useSettingsStore()
const screenshots = computed(() => (current.value?.screenshots ?? []).map(shot => ({
  url: catalogScreenshotURL(shot, settings.theme === 'dark'),
  caption: catalogScreenshotCaption(shot, gettext.current),
})))
const renderedReadme = computed(() => (readme.value ? marked.parse(readme.value) as string : ''))

const canInstall = computed(() => Boolean(current.value?.installable_release)
  && (!current.value?.installed_version || current.value?.update_available))

const links = computed(() => [
  { key: 'homepage', icon: GlobalOutlined, label: $gettext('Homepage'), url: current.value?.homepage_url },
  { key: 'repository', icon: LinkOutlined, label: $gettext('Repository'), url: current.value?.repository_url },
  { key: 'release_notes', icon: LinkOutlined, label: $gettext('Release notes'), url: current.value?.installable_release?.release_notes_url },
].filter(link => Boolean(link.url)))

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
    ...(recommendedMb.value > 0
      ? [{ key: 'memory', label: $gettext('Recommended memory'), value: formatMemory(recommendedMb.value), mono: false }]
      : []),
  ]
  return items
})

// Every version this node can install, each one can be picked.
const releases = computed(() => installableReleases(current.value))

// The notes worth reading before an install: every version newer than the
// installed one up to the offered release, or the offered release alone.
const releaseNotes = computed(() => {
  const entry = current.value
  const offered = entry?.installable_release
  if (!entry || !offered)
    return []
  const installedVersion = entry.installed_version
  const candidates = installedVersion && entry.update_available
    ? releases.value.filter(release => compareVersions(release.version, installedVersion) > 0
      && compareVersions(release.version, offered.version) <= 0)
    : [offered]
  return candidates
    .filter(release => release.notes)
    .map(release => ({
      version: release.version,
      date: release.released_at?.slice(0, 10),
      html: marked.parse(release.notes ?? '') as string,
    }))
})

function releaseAction(version: string) {
  const installedVersion = current.value?.installed_version
  if (!installedVersion)
    return $gettext('Install')
  return compareVersions(version, installedVersion) < 0 ? $gettext('Install this version') : $gettext('Update')
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
        v-if="offer && !current?.update_available"
        type="primary"
        :loading="replacingId === offer.entry.id"
        @click="confirmReplace(offer)"
      >
        {{ trustedOfferAction(offer) }}
      </AButton>
      <AButton
        v-else
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
            <PluginIcon :src="current.icon_url" :name="name" :size="48" />
            <div class="min-w-0 flex-1">
              <div class="pill-row mb-2">
                <ATooltip :title="trust.hint()">
                  <span class="pill" :class="trust.tone">{{ trust.label() }}</span>
                </ATooltip>
                <span
                  v-for="capability in current.capabilities ?? []"
                  :key="capability"
                  class="pill is-accent"
                >
                  {{ capabilityLabel(capability) }}
                </span>
                <ATooltip v-if="channel !== 'stable'" :title="channelHint(channel)">
                  <span class="pill" :class="channel === 'beta' ? 'is-warning' : 'is-purple'">{{ channelLabel(channel) }}</span>
                </ATooltip>
              </div>
              <p class="overview-description">
                {{ description || $gettext('No description provided.') }}
              </p>
            </div>
          </div>

          <AAlert
            v-if="lowMemory"
            type="warning"
            show-icon
            :title="memoryWarning(recommendedMb, systemMb)"
          />

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
                <PluginIcon
                  v-if="sourceIcon(current.source)"
                  :src="sourceIcon(current.source)"
                  :name="sourceName(current.source)"
                  :size="16"
                  class="detail-source-icon"
                />
                <span :title="current.source">{{ sourceName(current.source) }}</span>
              </dd>
            </div>
            <div v-if="current.categories?.length" class="detail-row">
              <dt>{{ $gettext('Categories') }}</dt>
              <dd class="pill-row">
                <span v-for="item in current.categories" :key="item" class="pill">{{ categoryLabel(item) }}</span>
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

          <section v-if="screenshots.length" class="overview-section">
            <div class="section-head">
              <h4 class="section-title">
                {{ $gettext('Screenshots') }}
              </h4>
            </div>
            <AImagePreviewGroup>
              <div class="screenshot-strip">
                <figure v-for="shot in screenshots" :key="shot.url" class="screenshot">
                  <AImage
                    :src="shot.url"
                    :alt="shot.caption || name"
                    :width="240"
                    :height="150"
                    referrerpolicy="no-referrer"
                    class="screenshot-image"
                  />
                  <figcaption v-if="shot.caption" class="screenshot-caption">
                    {{ shot.caption }}
                  </figcaption>
                </figure>
              </div>
            </AImagePreviewGroup>
          </section>

          <section class="overview-section">
            <div class="section-head">
              <h4 class="section-title">
                {{ $gettext('Requested permissions') }}
              </h4>
              <span v-if="permissions.length" class="section-count">{{ permissions.length }}</span>
            </div>
            <PermissionList :permissions="permissions" :reasons="reasons" />
          </section>

          <section v-if="releaseNotes.length" class="overview-section">
            <div class="section-head">
              <h4 class="section-title">
                {{ current.update_available ? $gettext('Changes in this update') : $gettext('Release notes') }}
              </h4>
            </div>
            <div class="notes-list">
              <article v-for="notes in releaseNotes" :key="notes.version">
                <div class="release-main">
                  <span class="release-version">v{{ notes.version }}</span>
                  <span v-if="notes.date" class="release-date">{{ notes.date }}</span>
                </div>
                <div v-dompurify-html="notes.html" class="plugin-readme release-notes-body" />
              </article>
            </div>
          </section>

          <section class="overview-section">
            <div class="section-head">
              <h4 class="section-title">
                {{ $gettext('Versions') }}
              </h4>
            </div>
            <ul class="release-list">
              <li
                v-for="release in releases"
                :key="release.version"
                class="release"
                :class="{ 'is-latest': release.version === current.installable_release?.version }"
              >
                <div class="release-main">
                  <span class="release-version">v{{ release.version }}</span>
                  <span v-if="release.version === current.installed_version" class="pill is-success">{{ $gettext('Installed') }}</span>
                  <span v-else-if="release.version === current.installable_release?.version" class="pill is-accent">{{ $gettext('Latest') }}</span>
                  <ATooltip v-if="releaseChannel(release) !== 'stable'" :title="channelHint(releaseChannel(release))">
                    <span class="pill" :class="releaseChannel(release) === 'beta' ? 'is-warning' : 'is-purple'">
                      {{ channelLabel(releaseChannel(release)) }}
                    </span>
                  </ATooltip>
                  <span v-if="release.released_at" class="release-date">{{ release.released_at.slice(0, 10) }}</span>
                  <a
                    v-if="release.release_notes_url"
                    :href="release.release_notes_url"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="pill is-link"
                  >
                    <LinkOutlined />
                    {{ $gettext('Release notes') }}
                  </a>
                </div>
                <AButton
                  v-if="release.version !== current.installed_version"
                  size="small"
                  @click="emit('install', current, release.version)"
                >
                  {{ releaseAction(release.version) }}
                </AButton>
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

.detail-source-icon {
  margin-right: 6px;
  vertical-align: -3px;
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

.notes-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.release-notes-body {
  margin-top: 6px;

  :deep(h1),
  :deep(h2),
  :deep(h3),
  :deep(h4) {
    margin: 10px 0 2px;
    font-size: 13px;
    font-weight: 600;
    color: var(--ant-color-text-secondary);
  }

  :deep(p) {
    margin: 4px 0;
  }

  :deep(ul),
  :deep(ol) {
    margin: 0;
    padding-left: 20px;
  }
}

.screenshot-strip {
  display: flex;
  gap: 12px;
  overflow-x: auto;
  padding-bottom: 4px;
}

.screenshot {
  flex: none;
  width: 240px;
  margin: 0;
}

.screenshot :deep(.screenshot-image) {
  border-radius: 6px;
  object-fit: cover;
  border: 1px solid var(--ant-color-border-secondary);
}

.screenshot-caption {
  margin-top: 6px;
  font-size: 12px;
  color: var(--ant-color-text-secondary);
}
</style>
