<script setup lang="ts">
import type { Component } from 'vue'
import type { SettingCatalogEntry } from './catalog'
import type { PreferenceSectionKey } from './sections'
import { LeftOutlined, RightOutlined } from '@antdv-next/icons'
import { useMediaQuery } from '@vueuse/core'
import { SETTING_ROW_CONTEXT } from '@/components/SettingPanel'
import gettext from '@/gettext'
import { useGlobalStore } from '@/pinia'
import {
  AccessTokens,
  AppSettings,
  AuthSettings,
  CertSettings,
  ExternalNotify,
  HealthCheckSettings,
  HTTPSettings,
  LogrotateSettings,
  NginxSettings,
  NodeSettings,
  OpenAISettings,
  PluginSettings,
  ServerSettings,
  TerminalSettings,
} from '@/views/preference/tabs'
import { buildSettingCatalog, findSettingEntry, settingEntryLabel } from './catalog'
import { PreferenceNav, PreferenceSaveBar, PreferenceSearch, PreferenceSectionList } from './components/Shell'
import {
  buildPreferenceGroups,
  buildPreferenceSections,
  DEFAULT_SECTION_KEY,
  findSectionForPath,
} from './sections'
import useSystemSettingsStore from './store'

const sectionComponents: Record<PreferenceSectionKey, Component> = {
  server: ServerSettings,
  app: AppSettings,
  node: NodeSettings,
  http: HTTPSettings,
  auth: AuthSettings,
  access_tokens: AccessTokens,
  cert: CertSettings,
  nginx: NginxSettings,
  plugin: PluginSettings,
  openai: OpenAISettings,
  health_check: HealthCheckSettings,
  external_notify: ExternalNotify,
  terminal: TerminalSettings,
  logrotate: LogrotateSettings,
}

const systemSettingsStore = useSystemSettingsStore()
const { changedPaths, isDirty, isSaving } = storeToRefs(systemSettingsStore)
const globalStore = useGlobalStore()
const isDemoResolved = ref(false)

void systemSettingsStore.getSettings()

const router = useRouter()
const route = useRoute()
const isNginxControlEditing = ref(false)
const highlightedPath = ref<string>()
const isMobile = useMediaQuery('(max-width: 600px)')

// Labels are rebuilt when the language changes.
const groups = computed(() => {
  void gettext.current
  return buildPreferenceGroups()
})

const sections = computed(() => {
  void gettext.current
  const showTerminal = isDemoResolved.value && !globalStore.isDemo
  return buildPreferenceSections().filter(section => section.key !== 'terminal' || showTerminal)
})

const catalog = computed(() => {
  void gettext.current
  return buildSettingCatalog()
})

function isSectionKey(value: string): value is PreferenceSectionKey {
  return value in sectionComponents
}

// The address is the source of truth for the open section, so links and
// the browser back button work on every screen size.
const routeSectionKey = computed(() => {
  const tab = route.query.tab?.toString()
  return tab && isSectionKey(tab) && sections.value.some(section => section.key === tab)
    ? tab
    : undefined
})

const activeKey = computed(() => routeSectionKey.value ?? DEFAULT_SECTION_KEY)

const activeSection = computed(() => sections.value.find(section => section.key === activeKey.value)
  ?? sections.value[0])

// Phones open on the section list and show one section at a time.
const isMobileList = computed(() => isMobile.value && !routeSectionKey.value)

// Set when a section was opened from the list, so back can return to it.
const hasListEntry = ref(false)

const changedPathSet = computed(() => new Set(changedPaths.value))

const dirtySections = computed(() => {
  const keys = new Set<PreferenceSectionKey>()
  for (const path of changedPaths.value) {
    const section = findSectionForPath(sections.value, path)
    if (section)
      keys.add(section.key)
  }
  return keys
})

const changedLabels = computed(() => {
  const labels = new Set<string>()
  for (const path of changedPaths.value) {
    const entry = findSettingEntry(catalog.value, path)
    labels.add(entry ? settingEntryLabel(entry) : path)
  }
  return [...labels]
})

const showSaveBar = computed(() => isDirty.value && !isNginxControlEditing.value)

provide(SETTING_ROW_CONTEXT, {
  highlightedPath,
  changedPaths: changedPathSet,
})

const sectionListeners = computed(() => activeSection.value.key === 'nginx'
  ? { controlEditing: (value: boolean) => { isNginxControlEditing.value = value } }
  : {})

watch(activeKey, () => {
  highlightedPath.value = undefined
})

// Only the tab is kept in the address; tables inside a section add their
// own query parameters and those must not follow the user to other sections.
async function openSection(key: PreferenceSectionKey) {
  if (key === routeSectionKey.value)
    return
  hasListEntry.value = isMobileList.value
  await router.push({ query: { tab: key } })
}

function backToList() {
  if (hasListEntry.value) {
    hasListEntry.value = false
    router.back()
    return
  }
  router.replace({ query: {} })
}

async function revealEntry(entry: SettingCatalogEntry) {
  await openSection(entry.section)
  highlightedPath.value = undefined
  await nextTick()
  highlightedPath.value = entry.path
}

onMounted(async () => {
  await globalStore.ensureDemoFlag()
  isDemoResolved.value = true
  // The terminal section is hidden in the demo, send its links to the default.
  if (globalStore.isDemo && route.query.tab === 'terminal')
    router.replace({ query: isMobile.value ? {} : { tab: DEFAULT_SECTION_KEY } })
})
</script>

<template>
  <ACard
    :title="$gettext('Preference')"
    class="preference-card"
    :styles="{ body: { padding: 0 } }"
  >
    <template #extra>
      <PreferenceSearch
        :entries="catalog"
        :sections="sections"
        @select="revealEntry"
      />
    </template>

    <div
      class="preference-layout"
      :class="{ 'is-compact': isMobile }"
    >
      <PreferenceNav
        v-if="!isMobile"
        :active-key="activeKey"
        :groups="groups"
        :sections="sections"
        :dirty-sections="dirtySections"
        @update:active-key="openSection"
      />

      <PreferenceSectionList
        v-if="isMobileList"
        :groups="groups"
        :sections="sections"
        :dirty-sections="dirtySections"
        @select="openSection"
      />

      <main
        v-else
        class="preference-content"
      >
        <button
          v-if="isMobile"
          type="button"
          class="preference-back"
          @click="backToList"
        >
          <LeftOutlined />
          {{ $gettext('Preference') }}
        </button>
        <header class="preference-section-head">
          <h2 class="preference-section-title">
            {{ activeSection.label }}
          </h2>
          <p class="preference-section-desc">
            {{ activeSection.description }}
            <RouterLink
              v-if="activeSection.link"
              :to="activeSection.link.to"
              class="preference-section-link"
            >
              {{ activeSection.link.label }}
              <RightOutlined class="text-xs" />
            </RouterLink>
          </p>
        </header>

        <component
          :is="sectionComponents[activeSection.key]"
          :key="activeSection.key"
          v-on="sectionListeners"
        />
      </main>
    </div>
  </ACard>

  <PreferenceSaveBar
    v-if="showSaveBar"
    :labels="changedLabels"
    :saving="isSaving"
    @save="systemSettingsStore.save()"
    @discard="systemSettingsStore.discard"
  />
</template>

<style lang="less" scoped>
.preference-layout {
  display: grid;
  grid-template-columns: 220px minmax(0, 1fr);
  min-height: 60vh;

  &.is-compact {
    grid-template-columns: minmax(0, 1fr);
  }
}

.preference-content {
  max-width: 900px;
  padding: 24px 32px 32px;
}

.is-compact .preference-content {
  padding: 16px;
}

.preference-section-head {
  margin-bottom: 20px;
}

.preference-section-title {
  margin: 0 0 4px;
  font-size: 18px;
  font-weight: 600;
  line-height: 1.4;
}

.preference-section-desc {
  margin: 0;
  color: var(--ant-color-text-secondary);
}

.preference-back {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin: 0 0 12px -4px;
  padding: 4px;
  border: none;
  background: transparent;
  color: var(--ant-color-primary);
  font: inherit;
  cursor: pointer;
}

.preference-section-link {
  margin-left: 4px;
  white-space: nowrap;
}
</style>
