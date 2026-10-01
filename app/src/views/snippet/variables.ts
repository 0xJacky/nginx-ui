import type { Variable } from '@/api/template'

export type VariableType = 'string' | 'boolean' | 'select'

export interface VariableOption {
  id: number
  value: string
  /** Label of the option per language. */
  labels: Record<string, string>
}

/** One variable of a snippet as the editor form holds it. */
export interface VariableRow {
  id: number
  key: string
  type: VariableType
  /** Label of the variable per language. */
  names: Record<string, string>
  value: string | boolean
  options: VariableOption[]
}

export const variableKeyPattern = /^[A-Z_]\w{0,63}$/i

let nextId = 0

export function newId() {
  return ++nextId
}

export function newOption(value = ''): VariableOption {
  return { id: newId(), value, labels: {} }
}

export function newVariable(key = ''): VariableRow {
  return { id: newId(), key, type: 'string', names: {}, value: '', options: [] }
}

function toType(type?: string): VariableType {
  return type === 'boolean' || type === 'select' ? type : 'string'
}

export function toRows(variables: Record<string, Variable> | undefined): VariableRow[] {
  return Object.entries(variables ?? {}).map(([key, v]) => {
    const type = toType(v.type)
    return {
      id: newId(),
      key,
      type,
      names: { ...v.name },
      value: type === 'boolean' ? Boolean(v.value) : String(v.value ?? ''),
      options: Object.entries(v.mask ?? {}).map(([value, labels]) => ({ id: newId(), value, labels: { ...labels } })),
    }
  })
}

export function toVariables(rows: VariableRow[]): Record<string, Variable> {
  const variables: Record<string, Variable> = {}
  for (const row of rows) {
    const variable: Variable = { type: row.type, name: row.names }
    if (row.type === 'boolean') {
      variable.value = row.value === true
    }
    else if (row.type === 'select') {
      variable.value = String(row.value).trim()
      variable.mask = Object.fromEntries(row.options.map(o => [o.value.trim(), o.labels]))
    }
    else {
      variable.value = String(row.value)
    }
    variables[row.key.trim()] = variable
  }
  return variables
}

/** Why the key of a variable cannot be saved, or an empty string. */
export function keyProblem(row: VariableRow, rows: VariableRow[]) {
  const key = row.key.trim()
  if (!key)
    return $gettext('Enter a key.')
  if (!variableKeyPattern.test(key))
    return $gettext('Use letters, digits and underscores, starting with a letter or underscore.')
  if (rows.some(other => other !== row && other.key.trim() === key))
    return $gettext('Another variable uses this key.')
  return ''
}

/** Why the options of a select cannot be saved, or an empty string. */
export function optionsProblem(row: VariableRow) {
  if (row.type !== 'select')
    return ''
  const values = row.options.map(o => o.value.trim())
  if (values.length === 0)
    return $gettext('Add at least one option.')
  if (values.includes(''))
    return $gettext('Every option needs a value.')
  if (new Set(values).size !== values.length)
    return $gettext('Option values must be unique.')
  if (!values.includes(String(row.value).trim()))
    return $gettext('Choose the default option.')
  return ''
}

export function hasVariableProblems(rows: VariableRow[]) {
  return rows.some(row => keyProblem(row, rows) || optionsProblem(row))
}

/** How the content refers to a variable. */
export function placeholderOf(key: string) {
  return `{{ .${key.trim()} }}`
}
