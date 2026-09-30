<script setup lang="ts">
import type { ComponentPublicInstance } from 'vue'
import { AppstoreOutlined, DeleteOutlined, EditOutlined, HolderOutlined, InfoCircleOutlined, PlusOutlined } from '@antdv-next/icons'
import { useSortable } from '@vueuse/integrations/useSortable'
import { localizedText } from '@/api/plugin'
import { getMarketplaceSources, probeMarketplaceSource, saveMarketplaceSources } from '@/api/plugin_marketplace'
import gettext from '@/gettext'
import { getErrorMessage } from '@/lib/http'
import { rememberSources } from './sources'

type SourceState = 'idle' | 'checking' | 'available' | 'unavailable' | 'invalid'

interface SourceRow {
  key: number
  url: string
  /** Shows the address as an input rather than as text. */
  editing: boolean
  /** URL the state below belongs to. */
  probedUrl: string
  state: SourceState
  /** Localized names the catalog declares. */
  catalogName?: Record<string, string>
  /** Image the catalog declares; dropped when it fails to load. */
  catalogIcon: string
  plugins: number
  /** Why the source is unavailable, shown on hover. */
  detail: string
}

const emit = defineEmits<{
  saved: []
}>()

const open = defineModel<boolean>('open', { default: false })

const { message } = App.useApp()

const loading = ref(false)
const saving = ref(false)
const error = ref('')
const rows = ref<SourceRow[]>([])
const defaultSource = ref('')
const dragging = ref(false)
const cards = useTemplateRef<ComponentPublicInstance>('cards')
let nextKey = 0

const canRestoreDefault = computed(() => Boolean(defaultSource.value)
  && !rows.value.some(row => row.url.trim() === defaultSource.value))

// Drag to reorder by the handle, so the address stays selectable.
useSortable(cards, rows, {
  handle: '.source-handle',
  ghostClass: 'source-ghost',
  chosenClass: 'source-chosen',
  animation: 220,
  easing: 'cubic-bezier(0.2, 0, 0, 1)',
  watchElement: true,
  onStart: () => {
    dragging.value = true
  },
  onEnd: () => {
    dragging.value = false
  },
})

function newRow(url = '', editing = false): SourceRow {
  return { key: nextKey++, url, editing, probedUrl: '', state: 'idle', catalogIcon: '', plugins: 0, detail: '' }
}

function isWebAddress(value: string) {
  try {
    const parsed = new URL(value)
    return (parsed.protocol === 'https:' || parsed.protocol === 'http:') && Boolean(parsed.host) && !/\s/.test(value)
  }
  catch {
    return false
  }
}

// Reads the catalog behind a row once per URL; a later edit wins over a
// check still running for the old URL.
async function probe(row: SourceRow) {
  const url = row.url.trim()
  if (url === row.probedUrl)
    return

  row.probedUrl = url
  row.catalogName = undefined
  row.catalogIcon = ''
  row.detail = ''
  if (!url) {
    row.state = 'idle'
    return
  }
  if (!isWebAddress(url)) {
    row.state = 'invalid'
    row.detail = $gettext('Enter a valid web address')
    return
  }

  row.state = 'checking'
  try {
    const result = await probeMarketplaceSource(url)
    if (row.probedUrl !== url)
      return
    row.state = result.reachable ? 'available' : 'unavailable'
    row.catalogName = result.catalog_name
    row.catalogIcon = result.catalog_icon ?? ''
    row.plugins = result.plugins
    row.detail = result.error ?? ''
  }
  catch (e) {
    if (row.probedUrl !== url)
      return
    row.state = 'unavailable'
    row.detail = getErrorMessage(e, $gettext('The source cannot be checked'))
  }
}

function hostOf(url: string) {
  try {
    return new URL(url).host
  }
  catch {
    return ''
  }
}

function titleOf(row: SourceRow) {
  return localizedText(row.catalogName, gettext.current)
    || hostOf(row.url.trim())
    || row.url.trim()
    || $gettext('New source')
}

function stateText(row: SourceRow) {
  switch (row.state) {
    case 'checking':
      return $gettext('Checking')
    case 'available':
      return $ngettext('%{count} plugin', '%{count} plugins', row.plugins, { count: String(row.plugins) })
    case 'unavailable':
      return $gettext('Unavailable')
    case 'invalid':
      return $gettext('Invalid address')
    default:
      return ''
  }
}

async function load() {
  loading.value = true
  error.value = ''
  rows.value = []
  try {
    const response = await getMarketplaceSources()
    rows.value = response.sources.map(source => newRow(source.url))
    defaultSource.value = response.default
  }
  catch (e) {
    error.value = getErrorMessage(e, $gettext('Failed to load the marketplace sources'))
  }
  finally {
    loading.value = false
  }
  rows.value.forEach(row => probe(row))
}

async function edit(row: SourceRow) {
  row.editing = true
  await nextTick()
  const list = cards.value?.$el as HTMLElement | undefined
  list?.querySelector<HTMLInputElement>(`[data-source-key="${row.key}"] input`)?.focus()
}

// Leaving the input checks the address; a valid one goes back to text.
function finishEdit(row: SourceRow) {
  probe(row)
  if (row.url.trim() && row.state !== 'invalid')
    row.editing = false
}

function addSource() {
  const row = newRow('', true)
  rows.value.push(row)
  edit(row)
}

function removeSource(index: number) {
  rows.value.splice(index, 1)
}

function restoreDefault() {
  const row = newRow(defaultSource.value)
  rows.value.push(row)
  probe(row)
}

async function save() {
  saving.value = true
  error.value = ''
  try {
    const response = await saveMarketplaceSources(rows.value
      .map(row => row.url.trim())
      .filter(Boolean))
    rememberSources(response.sources)
    message.success($gettext('Marketplace sources saved'))
    open.value = false
    emit('saved')
  }
  catch (e) {
    error.value = getErrorMessage(e, $gettext('Failed to save the marketplace sources'))
  }
  finally {
    saving.value = false
  }
}

watch(open, value => {
  if (value)
    load()
})
</script>

<template>
  <AModal
    v-model:open="open"
    :title="$gettext('Marketplace sources')"
    :width="640"
    :ok-text="$gettext('Save')"
    :cancel-text="$gettext('Cancel')"
    :confirm-loading="saving"
    @ok="save"
  >
    <Transition name="source-fade">
      <AAlert
        v-if="error"
        type="error"
        show-icon
        class="mb-4"
        :title="error"
      />
    </Transition>

    <div class="sources-intro">
      <InfoCircleOutlined class="sources-intro-icon" />
      <span>{{ $gettext('Catalogs are merged in order, the first source that offers a plugin wins. Leave the list empty to use the official catalog.') }}</span>
    </div>

    <div class="sources-list">
      <template v-if="loading">
        <div v-for="n in 2" :key="n" class="source-card is-skeleton">
          <ASkeleton avatar active :title="{ width: '40%' }" :paragraph="{ rows: 1, width: '75%' }" />
        </div>
      </template>

      <TransitionGroup
        v-show="!loading"
        ref="cards"
        tag="div"
        :name="dragging ? '' : 'source'"
        class="sources-cards"
      >
        <div
          v-for="(row, index) in rows"
          :key="row.key"
          class="source-card"
          :class="`is-${row.state}`"
          :data-source-key="row.key"
        >
          <HolderOutlined class="source-handle" :aria-label="$gettext('Drag to reorder')" />

          <div class="source-tile" :class="{ 'has-image': row.catalogIcon }">
            <Transition name="source-fade" mode="out-in">
              <img
                v-if="row.catalogIcon"
                :key="row.catalogIcon"
                :src="row.catalogIcon"
                alt=""
                class="source-image"
                @error="row.catalogIcon = ''"
              >
              <AppstoreOutlined v-else />
            </Transition>
          </div>

          <div class="source-main">
            <div class="source-head">
              <Transition name="source-fade" mode="out-in">
                <span
                  :key="row.catalogName ? 'catalog' : 'address'"
                  class="source-title"
                  :class="{ 'is-placeholder': !row.url.trim() || row.state === 'invalid' }"
                >{{ titleOf(row) }}</span>
              </Transition>
              <ATag v-if="row.url.trim() === defaultSource" color="blue" :bordered="false" class="source-tag">
                {{ $gettext('Official') }}
              </ATag>
            </div>

            <Transition name="source-swap" mode="out-in">
              <AInput
                v-if="row.editing"
                key="input"
                v-model:value="row.url"
                size="small"
                class="source-input"
                placeholder="https://example.com/plugins/index.json"
                @blur="finishEdit(row)"
                @press-enter="finishEdit(row)"
              />
              <button
                v-else
                key="text"
                type="button"
                class="source-url"
                :title="row.url"
                :aria-label="$gettext('Edit address')"
                @click="edit(row)"
              >
                <span class="source-url-text">{{ row.url }}</span>
                <EditOutlined class="source-url-icon" />
              </button>
            </Transition>
          </div>

          <div class="source-side">
            <Transition name="source-fade" mode="out-in">
              <span v-if="row.state !== 'idle'" :key="row.state" class="source-status">
                <ATooltip :title="row.detail || undefined">
                  <span class="source-pill">
                    <span class="source-dot" />
                    {{ stateText(row) }}
                  </span>
                </ATooltip>
              </span>
            </Transition>

            <AButton
              danger
              type="text"
              size="small"
              class="source-remove"
              :aria-label="$gettext('Remove')"
              @click="removeSource(index)"
            >
              <template #icon>
                <DeleteOutlined />
              </template>
            </AButton>
          </div>
        </div>
      </TransitionGroup>

      <Transition name="source-fade">
        <div v-if="rows.length === 0 && !loading" class="sources-empty">
          <div class="source-tile is-available">
            <AppstoreOutlined />
          </div>
          <div>
            <div class="sources-empty-title">
              {{ $gettext('Using the official catalog') }}
            </div>
            <div class="sources-empty-hint">
              {{ $gettext('Add a source to offer plugins from another catalog as well.') }}
            </div>
          </div>
        </div>
      </Transition>

      <button type="button" class="sources-add" @click="addSource">
        <PlusOutlined />
        <span>{{ $gettext('Add source') }}</span>
      </button>

      <Transition name="source-fade">
        <div v-if="canRestoreDefault && !loading" class="sources-official">
          <AButton type="link" size="small" @click="restoreDefault">
            {{ $gettext('Add the official catalog') }}
          </AButton>
        </div>
      </Transition>
    </div>
  </AModal>
</template>

<style scoped>
.sources-intro {
  display: flex;
  gap: 8px;
  align-items: flex-start;
  margin-bottom: 16px;
  padding: 10px 12px;
  border-radius: var(--ant-border-radius-lg, 8px);
  font-size: 13px;
  line-height: 1.6;
  color: var(--ant-color-text-secondary);
  background: var(--ant-color-fill-quaternary);
}

.sources-intro-icon {
  margin-top: 4px;
  color: var(--ant-color-primary);
}

.sources-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.sources-cards {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.sources-cards:empty {
  display: none;
}

/* A source card, tinted by the state of its catalog */
.source-card {
  --state-color: var(--ant-color-text-tertiary);
  --state-bg: var(--ant-color-fill-tertiary);

  position: relative;
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 12px 12px 12px 8px;
  border: 1px solid var(--ant-color-border-secondary);
  border-radius: 12px;
  background: var(--ant-color-bg-container);
  transition:
    border-color 0.2s ease,
    box-shadow 0.25s ease,
    transform 0.25s cubic-bezier(0.2, 0, 0, 1);
}

.source-card.is-checking {
  --state-color: var(--ant-color-primary);
  --state-bg: var(--ant-color-primary-bg);
}

.source-card.is-available {
  --state-color: var(--ant-color-success);
  --state-bg: var(--ant-color-success-bg);
}

.source-card.is-unavailable {
  --state-color: var(--ant-color-error);
  --state-bg: var(--ant-color-error-bg);
}

.source-card.is-invalid {
  --state-color: var(--ant-color-warning);
  --state-bg: var(--ant-color-warning-bg);
}

.source-card:not(.is-skeleton):hover,
.source-card:focus-within {
  border-color: var(--ant-color-border);
  box-shadow: 0 6px 16px -8px rgba(0, 0, 0, 0.18);
  transform: translateY(-1px);
}

.source-card.is-skeleton {
  padding: 16px 16px 12px;
}

.source-handle {
  align-self: stretch;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  margin-right: -6px;
  cursor: grab;
  color: var(--ant-color-text-quaternary);
  opacity: 0.5;
  transition: opacity 0.2s ease, color 0.2s ease;
}

.source-card:hover .source-handle,
.source-card:focus-within .source-handle {
  opacity: 1;
}

.source-handle:hover {
  color: var(--ant-color-text-secondary);
}

.source-tile {
  position: relative;
  display: flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: 10px;
  font-size: 18px;
  color: var(--state-color);
  background: var(--state-bg);
  transition: color 0.3s ease, background-color 0.3s ease;
}

.source-tile.has-image {
  background: var(--ant-color-bg-container);
  box-shadow:
    inset 0 0 0 1px var(--ant-color-border-secondary),
    0 0 0 2px var(--state-bg);
}

.source-image {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  object-fit: contain;
}

.sources-empty .source-tile {
  color: var(--ant-color-success);
  background: var(--ant-color-success-bg);
}

.source-main {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.source-head {
  display: flex;
  gap: 8px;
  align-items: center;
  min-width: 0;
}

.source-title {
  overflow: hidden;
  font-size: 14px;
  font-weight: 600;
  color: var(--ant-color-text);
  white-space: nowrap;
  text-overflow: ellipsis;
}

.source-title.is-placeholder {
  font-weight: 500;
  color: var(--ant-color-text-tertiary);
}

.source-tag {
  flex: none;
  margin: 0;
}

.source-url {
  display: flex;
  gap: 6px;
  align-items: center;
  max-width: 100%;
  padding: 0;
  border: 0;
  cursor: text;
  font-family: var(--ant-font-family-code, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 12px;
  line-height: 24px;
  text-align: left;
  color: var(--ant-color-text-tertiary);
  background: none;
}

.source-url-text {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.source-url-icon {
  flex: none;
  opacity: 0;
  transition: opacity 0.2s ease;
}

.source-url:hover,
.source-url:focus-visible {
  color: var(--ant-color-primary);
}

.source-url:hover .source-url-icon,
.source-url:focus-visible .source-url-icon {
  opacity: 1;
}

.source-url:focus-visible {
  outline: none;
}

.source-input {
  font-family: var(--ant-font-family-code, ui-monospace, SFMono-Regular, Menlo, monospace);
  font-size: 12px;
}

.source-side {
  display: flex;
  flex: none;
  gap: 4px;
  align-items: center;
}

.source-pill {
  display: inline-flex;
  gap: 6px;
  align-items: center;
  padding: 1px 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 500;
  line-height: 20px;
  white-space: nowrap;
  color: var(--state-color);
  background: var(--state-bg);
}

.is-unavailable .source-pill,
.is-invalid .source-pill {
  cursor: help;
}

.source-dot {
  position: relative;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

.is-checking .source-dot::after {
  position: absolute;
  inset: 0;
  border-radius: 50%;
  background: currentColor;
  content: '';
  animation: source-ping 1.2s cubic-bezier(0, 0, 0.2, 1) infinite;
}

.source-remove {
  opacity: 0.45;
  transition: opacity 0.2s ease;
}

.source-card:hover .source-remove,
.source-card:focus-within .source-remove {
  opacity: 1;
}

/* Touch screens have no hover, so the controls show in full */
@media (hover: none) {
  .source-handle,
  .source-remove {
    opacity: 1;
  }
}

.source-ghost {
  border-style: dashed;
  border-color: var(--ant-color-primary);
  background: var(--ant-color-primary-bg);
}

.source-ghost > * {
  opacity: 0.35;
}

.source-chosen {
  box-shadow: 0 12px 28px -10px rgba(0, 0, 0, 0.3);
}

.sources-empty {
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 14px 16px;
  border: 1px solid var(--ant-color-border-secondary);
  border-radius: 12px;
}

.sources-empty-title {
  font-weight: 600;
  color: var(--ant-color-text);
}

.sources-empty-hint {
  font-size: 12px;
  color: var(--ant-color-text-tertiary);
}

.sources-add {
  display: flex;
  gap: 8px;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 44px;
  border: 1px dashed var(--ant-color-border);
  border-radius: 12px;
  cursor: pointer;
  font-size: 14px;
  color: var(--ant-color-text-secondary);
  background: transparent;
  transition:
    color 0.2s ease,
    border-color 0.2s ease,
    background-color 0.2s ease;
}

.sources-add:hover,
.sources-add:focus-visible {
  outline: none;
  border-color: var(--ant-color-primary);
  color: var(--ant-color-primary);
  background: var(--ant-color-primary-bg);
}

.sources-add:active {
  transform: scale(0.995);
}

.sources-official {
  margin-top: -4px;
  text-align: center;
}

/* Cards slide in, collapse out and glide to a new place */
.source-enter-active {
  transition:
    opacity 0.3s ease,
    transform 0.35s cubic-bezier(0.2, 0, 0, 1);
}

.source-leave-active {
  position: absolute;
  right: 0;
  left: 0;
  transition:
    opacity 0.2s ease,
    transform 0.25s ease;
}

.source-enter-from {
  opacity: 0;
  transform: translateY(-10px) scale(0.97);
}

.source-leave-to {
  opacity: 0;
  transform: translateX(24px) scale(0.97);
}

.source-move {
  transition: transform 0.35s cubic-bezier(0.2, 0, 0, 1);
}

.source-fade-enter-active,
.source-fade-leave-active {
  transition: opacity 0.18s ease, transform 0.18s ease;
}

.source-fade-enter-from,
.source-fade-leave-to {
  opacity: 0;
  transform: translateY(3px);
}

.source-swap-enter-active,
.source-swap-leave-active {
  transition: opacity 0.14s ease;
}

.source-swap-enter-from,
.source-swap-leave-to {
  opacity: 0;
}

@keyframes source-ping {
  0% {
    opacity: 0.7;
    transform: scale(1);
  }

  100% {
    opacity: 0;
    transform: scale(2.8);
  }
}

/* Narrow screens: the status moves under the address, remove stays in the corner */
@media (max-width: 575px) {
  .source-card {
    flex-wrap: wrap;
    row-gap: 6px;
  }

  .source-main {
    padding-right: 28px;
  }

  .source-side {
    display: contents;
  }

  .source-status {
    order: 1;
    flex-basis: 100%;
    padding-left: 74px;
  }

  .source-remove {
    position: absolute;
    top: 8px;
    right: 8px;
  }

  .sources-intro {
    font-size: 12px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .source-card,
  .source-enter-active,
  .source-leave-active,
  .source-move,
  .source-fade-enter-active,
  .source-fade-leave-active,
  .source-swap-enter-active,
  .source-swap-leave-active {
    transition: none;
  }

  .is-checking .source-dot::after {
    animation: none;
  }
}
</style>
