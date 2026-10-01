import type { VariableRow, VariableType } from './variables'
import { newVariable, placeholderOf, variableKeyPattern } from './variables'

/** One {{ }} action of a snippet body. */
export interface TemplateAction {
  start: number
  end: number
  /** The variable the action shows or tests, or the one of the block it closes. */
  key?: string
  /** Every variable the action refers to. */
  keys: string[]
}

// Values the config template panel gives every template.
const providedKeys = new Set(['HTTPPORT', 'UNIXSOCKET', 'NGINXUIUPSTREAM', 'HTTP01PORT'])

const fieldPattern = /(?:^|[\s(|,=])\.([A-Z_]\w*)/gi

/**
 * The actions of a body. An else or end action belongs to the variable its
 * block tests, so a whole if block reads as one variable.
 */
export function scanActions(content: string): TemplateAction[] {
  const actions: TemplateAction[] = []
  const blocks: (string | undefined)[] = []
  for (const match of content.matchAll(/\{\{(.*?)\}\}/gs)) {
    const inner = match[1].replace(/^-|-$/g, '').trim()
    const keys = [...inner.matchAll(fieldPattern)].map(m => m[1]).filter(k => !providedKeys.has(k))
    const start = match.index!
    const action: TemplateAction = { start, end: start + match[0].length, key: keys[0], keys }
    if (/^(?:if|with|range|block|define)\b/.test(inner)) {
      blocks.push(keys[0])
    }
    else if (/^else\b/.test(inner)) {
      action.key ??= blocks.at(-1)
    }
    else if (/^end\b/.test(inner)) {
      action.key = blocks.pop()
    }
    actions.push(action)
  }
  return actions
}

/** The variable keys the actions of a body refer to. */
export function referencedKeys(content: string) {
  return new Set(scanActions(content).flatMap(a => a.keys))
}

/** Colors of the variables, in the order of the list. */
export const palette = ['#3b82f6', '#a855f7', '#10b981', '#ec4899', '#06b6d4', '#f97316', '#84cc16', '#6366f1']

/** Color of a variable the body refers to without declaring it. */
export const undeclaredColor = '#f59e0b'

export const paletteSize = palette.length

/** The color slot of each declared variable, in the order of the list. */
export function colorSlots(rows: VariableRow[]) {
  return new Map(rows.map((row, index) => [row.key.trim(), index % paletteSize]))
}

/** The color of a variable, or of a variable that is not declared. */
export function colorOf(slots: Map<string, number>, key: string) {
  const slot = slots.get(key)
  return slot === undefined ? undeclaredColor : palette[slot]
}

/** The class of a variable color, or of a variable that is not declared. */
export function colorClass(slots: Map<string, number>, key?: string) {
  if (!key)
    return ''
  const slot = slots.get(key)
  return slot === undefined ? 'snippet-var-undeclared' : `snippet-var-c${slot}`
}

/** A body split into plain text and actions, to show it highlighted. */
export function segments(content: string) {
  const parts: { text: string, key?: string, isAction: boolean }[] = []
  let at = 0
  for (const action of scanActions(content)) {
    if (action.start > at)
      parts.push({ text: content.slice(at, action.start), isAction: false })
    parts.push({ text: content.slice(action.start, action.end), key: action.key, isAction: true })
    at = action.end
  }
  if (at < content.length)
    parts.push({ text: content.slice(at), isAction: false })
  return parts
}

function camelCase(text: string) {
  const words = text.split(/[^a-z0-9]+/i).filter(Boolean)
  return words.map((w, i) => i === 0 ? w.toLowerCase() : w[0].toUpperCase() + w.slice(1).toLowerCase()).join('')
}

function uniqueKey(base: string, rows: VariableRow[]) {
  let key = variableKeyPattern.test(base) ? base : 'value'
  const taken = new Set(rows.map(r => r.key.trim()))
  for (let n = 2; taken.has(key); n++)
    key = `${variableKeyPattern.test(base) ? base : 'value'}${n}`
  return key
}

/**
 * A variable for a value selected in the body: the key comes from the
 * directive of the line, the type from the value, and the value becomes
 * the default.
 */
export function guessVariable(value: string, line: string, rows: VariableRow[]): VariableRow {
  const directive = line.trim().split(/\s+/)[0]?.replace(/[;{}]/g, '') ?? ''
  const isSwitch = /^(?:on|off)$/i.test(value)
  let base = camelCase(directive)
  if (/^[a-z][a-z0-9+.-]*:\/\//i.test(value))
    base = directive === 'return' || directive === 'rewrite' ? 'target' : directive === 'proxy_pass' ? 'backend' : 'url'
  if (!base || base === value.toLowerCase())
    base = 'value'
  const row = newVariable(uniqueKey(base, rows))
  row.names = { en: labelFromKey(row.key) }
  row.type = isSwitch ? 'boolean' : 'string'
  row.value = isSwitch ? value.toLowerCase() === 'on' : value
  return row
}

/** An English label from a key, such as "Client max body size". */
export function labelFromKey(key: string) {
  const words = key.replace(/([a-z\d])([A-Z])/g, '$1 $2').replace(/_/g, ' ').trim().toLowerCase()
  return words ? words[0].toUpperCase() + words.slice(1) : ''
}

/** What replaces a selected value once it is a variable. */
export function replacementOf(key: string, type: VariableType) {
  return type === 'boolean' ? `{{ if .${key} }}on{{ else }}off{{ end }}` : placeholderOf(key)
}
