<script setup lang="ts">
import type { Ace, Editor } from 'ace-builds'
import type { VariableRow, VariableType } from '../variables'
import { DownOutlined } from '@antdv-next/icons'
import { useDebounceFn } from '@vueuse/core'
import ace from 'ace-builds'
import CodeEditor from '@/components/CodeEditor'
import { localize, useSnippetDescription } from '../description'
import { colorClass, colorOf, colorSlots, guessVariable, labelFromKey, replacementOf, scanActions } from '../template'
import { keyProblem, newVariable } from '../variables'
import LocalizedInput from './LocalizedInput.vue'
import 'ace-builds/src-noconflict/ext-language_tools'

const props = defineProps<{
  rows: VariableRow[]
  height?: string
  // Shows the content with its variables, without editing tools.
  readonly?: boolean
  // Grows with the content between these line counts instead of a fixed height.
  lines?: [number, number]
}>()

const emit = defineEmits<{
  addVariable: [row: VariableRow]
}>()

const content = defineModel<string>({ required: true })
// Key of the variable the list has open.
const activeKey = defineModel<string>('active')

const { current: language } = useSnippetDescription()
const wrapper = useTemplateRef<HTMLElement>('wrapper')
const editor = shallowRef<Editor>()

const slots = computed(() => colorSlots(props.rows))
const actions = computed(() => scanActions(content.value))
const declared = computed(() => props.rows.filter(r => r.key.trim() && !keyProblem(r, props.rows)))

function rowOf(key: string) {
  return props.rows.find(r => r.key.trim() === key)
}

function labelOf(row: VariableRow) {
  return localize(row.names, language.value) || row.key
}

const typeLabels = computed<Record<VariableType, string>>(() => ({
  string: $gettext('Text'),
  boolean: $gettext('Switch'),
  select: $gettext('Select'),
}))

function defaultOf(row: VariableRow) {
  if (row.type === 'boolean')
    return row.value ? $gettext('On') : $gettext('Off')
  if (row.type === 'select') {
    const option = row.options.find(o => o.value === row.value)
    return option ? localize(option.labels, language.value) || option.value : String(row.value)
  }
  return String(row.value)
}

function rangeOf(start: number, end: number) {
  const doc = editor.value!.session.doc
  const from = doc.indexToPosition(start, 0)
  const to = doc.indexToPosition(end, 0)
  return new ace.Range(from.row, from.column, to.row, to.column)
}

// Variable colors behind the actions of the body.
let markerIds: number[] = []

function refreshMarkers() {
  const session = editor.value?.session
  if (!session)
    return
  markerIds.forEach(id => session.removeMarker(id))
  markerIds = []
  for (const action of actions.value) {
    const cls = colorClass(slots.value, action.key)
    if (!cls)
      continue
    const active = action.key === activeKey.value ? ' snippet-var-active' : ''
    markerIds.push(session.addMarker(rangeOf(action.start, action.end), `snippet-var ${cls}${active}`, 'text', false))
  }
}

watch([actions, slots, activeKey], refreshMarkers)

function actionAt(row: number, column: number) {
  const index = editor.value!.session.doc.positionToIndex({ row, column }, 0)
  return actions.value.find(a => a.key && index >= a.start && index < a.end)
}

function pointBelow(row: number, column: number) {
  const coords = editor.value!.renderer.textToScreenCoordinates(row, column)
  const box = wrapper.value!.getBoundingClientRect()
  return { x: coords.pageX - box.left, y: coords.pageY - box.top + editor.value!.renderer.lineHeight + 4 }
}

// The card shown while the pointer rests on a variable.
const hover = ref<{ key: string, x: number, y: number }>()
const isOverCard = ref(false)
const hideHover = useDebounceFn(() => {
  if (!isOverCard.value)
    hover.value = undefined
}, 200)

function onMouseMove(event: MouseEvent) {
  const position = editor.value!.renderer.screenToTextCoordinates(event.clientX, event.clientY)
  const action = actionAt(position.row, position.column)
  if (!action?.key) {
    hideHover()
    return
  }
  if (hover.value?.key === action.key)
    return
  const start = editor.value!.session.doc.indexToPosition(action.start, 0)
  hover.value = { key: action.key, ...pointBelow(start.row, start.column) }
}

function openVariable(key: string) {
  activeKey.value = key
  hover.value = undefined
}

function declare(key: string) {
  emit('addVariable', newVariable(key))
  openVariable(key)
}

// The button offered next to a value selected in the body.
const selection = ref<{ x: number, y: number, range: Ace.Range }>()

function updateSelection() {
  const instance = editor.value
  if (!instance)
    return
  const range = instance.getSelectionRange()
  const text = instance.session.getTextRange(range)
  if (range.isEmpty() || range.start.row !== range.end.row || !text.trim() || /\{\{|\}\}/.test(text) || text.length > 200
    || actionAt(range.start.row, range.start.column)) {
    selection.value = undefined
    return
  }
  selection.value = { range: range.clone(), ...pointBelow(range.end.row, range.end.column) }
}

const debouncedSelection = useDebounceFn(updateSelection, 120)

// The dialog that turns the selected value into a variable, or declares a
// new one where the cursor is. A tail marks the second case: the key and the
// tail replace the range.
const draft = ref<{ row: VariableRow, range: Ace.Range, tail?: string, x: number, y: number }>()
const draftKey = useTemplateRef<{ focus: () => void }>('draftKey')

// Opens the panel below a range, kept inside the editor.
function openDraft(row: VariableRow, range: Ace.Range, tail?: string) {
  const point = pointBelow(range.start.row, range.start.column)
  const width = wrapper.value!.clientWidth
  draft.value = { row, range, tail, x: Math.max(0, Math.min(point.x, width - 460)), y: point.y }
  nextTick(() => draftKey.value?.focus())
}

function cancelDraft() {
  draft.value = undefined
  editor.value?.focus()
}
const isNewDraft = computed(() => draft.value?.tail !== undefined)

const draftProblem = computed(() => draft.value ? keyProblem(draft.value.row, [...props.rows, draft.value.row]) : '')

function makeVariable() {
  const instance = editor.value
  const range = selection.value?.range ?? instance?.getSelectionRange()
  if (!instance || !range || range.isEmpty() || range.start.row !== range.end.row)
    return
  const value = instance.session.getTextRange(range).trim()
  openDraft(guessVariable(value, instance.session.getLine(range.start.row), props.rows), range.clone())
  selection.value = undefined
}

function changeDraftType(type: VariableType) {
  const row = draft.value!.row
  const text = isNewDraft.value ? '' : editor.value!.session.getTextRange(draft.value!.range).trim()
  row.type = type
  row.value = type === 'boolean' ? text.toLowerCase() === 'on' : text
}

// Declares a variable from the completion list. The word typed after the
// dot becomes its key.
function newVariableAtCursor(tail: string) {
  const instance = editor.value!
  const cursor = instance.getCursorPosition()
  const typed = /\w*$/.exec(instance.session.getLine(cursor.row).slice(0, cursor.column))![0]
  const row = newVariable(typed)
  row.names = typed ? { en: labelFromKey(typed) } : {}
  openDraft(row, new ace.Range(cursor.row, cursor.column - typed.length, cursor.row, cursor.column), tail)
}

function confirmDraft() {
  if (!draft.value || draftProblem.value)
    return
  const { row, range, tail } = draft.value
  row.key = row.key.trim()
  editor.value!.session.replace(range, tail === undefined ? replacementOf(row.key, row.type) : row.key + tail)
  emit('addVariable', row)
  activeKey.value = row.key
  draft.value = undefined
  editor.value!.focus()
}

// Inserts a variable at the cursor. A switch wraps the selection in its if
// block.
function insertVariable(key: string) {
  const instance = editor.value
  const row = rowOf(key)
  if (!instance || !row)
    return
  const selected = instance.getSelectedText()
  instance.insert(row.type === 'boolean' ? `{{ if .${key} }}${selected}{{ end }}` : `{{ .${key} }}`)
  instance.focus()
}

const insertItems = computed(() => declared.value.map(row => ({ key: row.key.trim(), label: labelOf(row) })))

// Completes variable keys after {{ .
const completer: Ace.Completer = {
  identifierRegexps: [/\w/],
  triggerCharacters: ['.'],
  getCompletions(_editor, session, position, _prefix, callback) {
    const line = session.getLine(position.row)
    const before = line.slice(0, position.column)
    if (!/\{\{-?\s*(?:(?:if|with|range|not|and|or|else\s+if)\s+)*\.\w*$/.test(before)) {
      callback(null, [])
      return
    }
    const after = line.slice(position.column).replace(/^\w*/, '')
    const isBlock = /\{\{-?\s*(?:if|with|range|else\s+if)\s/.test(before)
    // Closes the action, or keeps a space before a closing that is there.
    const tail = /^\s/.test(after) ? '' : /^-?\}\}/.test(after) ? ' ' : isBlock ? '' : ' }}'
    const known = new Set(declared.value.map(row => row.key.trim()))
    const typed = /\w*$/.exec(before)![0]
    callback(null, [
      ...declared.value.map((row, index) => ({
        caption: row.key.trim(),
        value: row.key.trim() + tail,
        meta: labelOf(row),
        score: 1000 - index,
      })),
      // Always offered, below the declared variables.
      ...(known.has(typed)
        ? []
        : [{
            caption: $gettext('New Variable…'),
            // Not empty, or the list takes it for no suggestion.
            value: typed || '.',
            score: 0,
            skipFilter: true,
            completer: {
              getCompletions: (_e, _s, _p, _x, done) => done(null, []),
              insertMatch: () => newVariableAtCursor(tail),
            },
          } as Ace.Completion]),
    ])
  },
}

function init(instance: Editor) {
  editor.value = instance
  if (props.lines)
    instance.setOptions({ minLines: props.lines[0], maxLines: props.lines[1] })
  // The editor gets its content after it starts, and positions follow it.
  instance.on('change', () => nextTick(refreshMarkers))
  instance.container.addEventListener('mousemove', onMouseMove)
  instance.container.addEventListener('mouseleave', () => hideHover())
  nextTick(refreshMarkers)
  instance.session.on('changeScrollTop', () => {
    selection.value = undefined
    hover.value = undefined
  })
  if (props.readonly)
    return
  instance.setOptions({ enableBasicAutocompletion: true, enableLiveAutocompletion: true })
  ;(instance as Editor & { completers: Ace.Completer[] }).completers = [completer]
  instance.commands.addCommand({
    name: 'snippetMakeVariable',
    bindKey: { win: 'Ctrl-E', mac: 'Command-E' },
    exec: makeVariable,
  })
  instance.selection.on('changeSelection', debouncedSelection)
  instance.container.addEventListener('click', (event: MouseEvent) => {
    const position = instance.renderer.screenToTextCoordinates(event.clientX, event.clientY)
    const action = actionAt(position.row, position.column)
    if (action?.key && rowOf(action.key))
      activeKey.value = action.key
  })
}

const hoverRow = computed(() => hover.value ? rowOf(hover.value.key) : undefined)
const isMac = /Mac|iPhone|iPad/.test(navigator.platform)
</script>

<template>
  <div class="flex flex-col gap-2">
    <div
      v-if="!readonly"
      class="flex flex-wrap items-center gap-2"
    >
      <slot name="toolbar" />
      <ADropdown
        :disabled="insertItems.length === 0"
        :menu="{ items: insertItems.map(i => ({ key: i.key, label: i.label })), onClick: ({ key }: { key: string | number }) => insertVariable(String(key)) }"
        :trigger="['click']"
      >
        <AButton size="small">
          {{ $gettext('Insert Variable') }}
          <DownOutlined />
        </AButton>
      </ADropdown>
      <span class="hint ml-auto text-xs max-sm:hidden">
        {{ $gettext('Type %{open} to complete a variable, or select a value and press %{keys} to make it one.', { open: '{{ .', keys: isMac ? '⌘E' : 'Ctrl+E' }) }}
      </span>
    </div>

    <div
      ref="wrapper"
      class="snippet-code relative"
    >
      <CodeEditor
        v-model:content="content"
        :default-height="lines ? '0px' : height"
        :readonly="readonly"
        @init="init"
      />

      <div
        v-if="hover && !draft"
        class="hover-card"
        :style="{ left: `${Math.max(0, hover.x)}px`, top: `${hover.y}px` }"
        @mouseenter="isOverCard = true"
        @mouseleave="isOverCard = false; hideHover()"
      >
        <template v-if="hoverRow">
          <div class="flex items-center justify-between gap-3">
            <span class="font-medium">{{ labelOf(hoverRow) }}</span>
            <ATag
              :bordered="false"
              class="m-0"
            >
              {{ typeLabels[hoverRow.type] }}
            </ATag>
          </div>
          <div class="mt-1 flex justify-between gap-3">
            <span class="hint shrink-0">{{ $gettext('Default Value') }}</span>
            <span class="min-w-0 truncate">{{ defaultOf(hoverRow) || '""' }}</span>
          </div>
          <AButton
            v-if="!readonly"
            type="link"
            size="small"
            class="mt-1 p-0"
            @click="openVariable(hover.key)"
          >
            {{ $gettext('Edit Variable') }}
          </AButton>
        </template>
        <template v-else>
          <div
            class="font-mono"
            :style="{ color: colorOf(slots, hover.key) }"
          >
            .{{ hover.key }}
          </div>
          <div class="hint mt-1">
            {{ $gettext('This variable is not declared, so it has no value when the snippet is inserted.') }}
          </div>
          <AButton
            v-if="!readonly"
            type="link"
            size="small"
            class="mt-1 p-0"
            @click="declare(hover.key)"
          >
            {{ $gettext('Declare Variable') }}
          </AButton>
        </template>
      </div>

      <div
        v-if="selection && !draft"
        class="selection-callout"
        :style="{ left: `${Math.max(0, selection.x - 60)}px`, top: `${selection.y}px` }"
      >
        <AButton
          type="primary"
          size="small"
          @mousedown.prevent
          @click="makeVariable"
        >
          {{ $gettext('Make Variable') }}
        </AButton>
        <span class="hint px-1 text-xs">{{ isMac ? '⌘E' : 'Ctrl+E' }}</span>
      </div>

      <div
        v-if="draft"
        class="draft-panel"
        :style="{ left: `${draft.x}px`, top: `${draft.y}px` }"
        role="dialog"
        :aria-label="isNewDraft ? $gettext('New Variable') : $gettext('Make Variable')"
        @keydown.esc.stop="cancelDraft"
      >
        <div class="mb-3 font-medium">
          {{ isNewDraft ? $gettext('New Variable') : $gettext('Make Variable') }}
        </div>
        <div class="flex flex-col gap-3">
          <div class="grid grid-cols-2 gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,1.3fr)_7rem]">
            <label class="flex min-w-0 flex-col gap-1">
              <span class="hint text-[13px]">{{ $gettext('Key') }}</span>
              <AInput
                v-model:value="draft.row.key"
                class="font-mono"
                placeholder="maxAge"
                :status="draft.row.key && draftProblem ? 'error' : undefined"
                @press-enter="confirmDraft"
              />
            </label>
            <label class="flex min-w-0 flex-col gap-1">
              <span class="hint text-[13px]">{{ $gettext('Label') }}</span>
              <LocalizedInput
                v-model="draft.row.names"
                :maxlength="100"
              />
            </label>
            <label class="flex min-w-0 flex-col gap-1">
              <span class="hint text-[13px]">{{ $gettext('Type') }}</span>
              <ASelect
                :value="draft.row.type"
                :options="[{ label: typeLabels.string, value: 'string' }, { label: typeLabels.boolean, value: 'boolean' }]"
                @change="changeDraftType($event as VariableType)"
              />
            </label>
          </div>
          <label class="flex flex-col gap-1">
            <span class="hint text-[13px]">{{ $gettext('Default Value') }}</span>
            <div
              v-if="draft.row.type === 'boolean'"
              class="flex h-8 items-center"
            >
              <ASwitch v-model:checked="draft.row.value as boolean" />
            </div>
            <AInput
              v-else
              v-model:value="draft.row.value as string"
              @press-enter="confirmDraft"
            />
          </label>
          <div
            v-if="draftProblem && draft.row.key"
            class="problem text-sm"
          >
            {{ draftProblem }}
          </div>
          <div class="hint text-sm">
            {{ isNewDraft
              ? $gettext('The variable is declared and inserted at the cursor.')
              : $gettext('The selected value becomes the default value, and the content refers to the variable instead.') }}
          </div>
        </div>
        <div class="mt-3 flex justify-end gap-2">
          <AButton
            size="small"
            @click="cancelDraft"
          >
            {{ $gettext('Cancel') }}
          </AButton>
          <AButton
            size="small"
            type="primary"
            :disabled="!!draftProblem"
            @click="confirmDraft"
          >
            {{ isNewDraft ? $gettext('Add Variable') : $gettext('Make Variable') }}
          </AButton>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped lang="less">
.hint {
  color: var(--ant-color-text-secondary);
}

.problem {
  color: var(--ant-color-error);
}

.hover-card {
  position: absolute;
  z-index: 10;
  width: 300px;
  max-width: calc(100% - 8px);
  padding: 10px 12px;
  border-radius: 8px;
  background: var(--ant-color-bg-elevated);
  color: var(--ant-color-text);
  box-shadow: var(--ant-box-shadow-secondary);
  font-size: 13px;
}

.draft-panel {
  position: absolute;
  z-index: 11;
  width: 450px;
  max-width: 100%;
  padding: 12px;
  border-radius: 8px;
  background: var(--ant-color-bg-elevated);
  color: var(--ant-color-text);
  box-shadow: var(--ant-box-shadow-secondary);
}

.selection-callout {
  position: absolute;
  z-index: 10;
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px;
  border-radius: 8px;
  background: var(--ant-color-bg-elevated);
  box-shadow: var(--ant-box-shadow-secondary);
}
</style>

<style lang="less">
// Markers live in the Ace DOM, so these rules are global.
.snippet-code .ace_marker-layer .snippet-var {
  position: absolute;
  border-radius: 3px;
  background: color-mix(in srgb, var(--snippet-var) 30%, transparent);
  box-shadow: inset 0 -2px 0 var(--snippet-var);
}

.snippet-code .ace_marker-layer .snippet-var-active {
  background: color-mix(in srgb, var(--snippet-var) 55%, transparent);
}

.snippet-code .ace_marker-layer .snippet-var-undeclared {
  --snippet-var: #f59e0b;
  background: color-mix(in srgb, var(--snippet-var) 18%, transparent);
  box-shadow: none;
  border-bottom: 2px dashed var(--snippet-var);
}

.snippet-code .ace_marker-layer {
  .snippet-var-c0 { --snippet-var: #3b82f6; }
  .snippet-var-c1 { --snippet-var: #a855f7; }
  .snippet-var-c2 { --snippet-var: #10b981; }
  .snippet-var-c3 { --snippet-var: #ec4899; }
  .snippet-var-c4 { --snippet-var: #06b6d4; }
  .snippet-var-c5 { --snippet-var: #f97316; }
  .snippet-var-c6 { --snippet-var: #84cc16; }
  .snippet-var-c7 { --snippet-var: #6366f1; }
}
</style>
