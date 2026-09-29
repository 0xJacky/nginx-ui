import type { DNSProviderField, DNSProviderForm, DNSProviderMethod } from '@/api/auto_cert'

/** Configuration map a field value is stored in. */
export type ConfigurationTarget = 'credentials' | 'additional'

/** Values of credential keys held back while another method is selected. */
export type MethodStash = Record<string, string>

function hasValue(value: string | undefined | null): boolean {
  return value !== undefined && value !== null && String(value).trim() !== ''
}

/** Credential fields of the form, in display order. */
export function credentialFields(form: DNSProviderForm | undefined): DNSProviderField[] {
  return (form?.fields ?? []).filter(field => (field.group ?? 'credential') === 'credential')
}

/** Setting fields of the form, in display order. */
export function settingFields(form: DNSProviderForm | undefined): DNSProviderField[] {
  return (form?.fields ?? []).filter(field => field.group === 'setting')
}

/** Methods worth offering, which is only when there is more than one. */
export function formMethods(form: DNSProviderForm | undefined): DNSProviderMethod[] {
  const methods = form?.methods ?? []
  return methods.length > 1 ? methods : []
}

/**
 * Credential fields shown for a method: its own fields plus those no method
 * lists. Without a method every credential field is shown.
 */
export function visibleCredentialFields(form: DNSProviderForm | undefined, methodName: string | undefined): DNSProviderField[] {
  const fields = credentialFields(form)
  const methods = formMethods(form)
  const method = methods.find(item => item.name === methodName)
  if (!method)
    return fields

  const listed = new Set(methods.flatMap(item => item.fields ?? []))
  const own = new Set(method.fields ?? [])
  return fields.filter(field => own.has(field.key) || !listed.has(field.key))
}

/** Method a name refers to among the offered methods. */
export function findMethod(form: DNSProviderForm | undefined, methodName: string | undefined): DNSProviderMethod | undefined {
  return formMethods(form).find(item => item.name === methodName)
}

/** True when the method has nothing for the user to fill in. */
export function methodNeedsNoInput(form: DNSProviderForm | undefined, methodName: string | undefined): boolean {
  const method = findMethod(form, methodName)
  if (!method || (method.fields ?? []).length > 0)
    return false
  return visibleCredentialFields(form, methodName).length === 0
}

function methodValueEntries(method: DNSProviderMethod): [string, string][] {
  return Object.entries(method.values ?? {})
}

/** True when every fixed value of the method is stored as is. */
function valuesMatch(method: DNSProviderMethod, credentials: Record<string, string> | undefined): boolean {
  const entries = methodValueEntries(method)
  return entries.length > 0 && entries.every(([key, value]) => credentials?.[key] === value)
}

function isBetterScore(score: [number, number], bestScore: [number, number], method: DNSProviderMethod, best: DNSProviderMethod): boolean {
  if (score[0] !== bestScore[0])
    return score[0] > bestScore[0]
  if (score[1] !== bestScore[1])
    return score[1] > bestScore[1]
  return !!method.recommended && !best.recommended
}

/**
 * Method to preselect. When editing, a method whose fixed values are all
 * stored wins, then the one whose keys hold the most values. Otherwise the
 * recommended one, otherwise the first.
 */
export function initialMethod(form: DNSProviderForm | undefined, credentials: Record<string, string> | undefined): string | undefined {
  const methods = formMethods(form)
  if (methods.length === 0)
    return undefined

  // Scores compare as [matched values, filled fields].
  let best: DNSProviderMethod | undefined
  let bestScore: [number, number] = [0, 0]
  for (const method of methods) {
    const matched = valuesMatch(method, credentials) ? methodValueEntries(method).length : 0
    const filled = (method.fields ?? []).filter(key => hasValue(credentials?.[key])).length
    if (matched === 0 && filled === 0)
      continue
    if (!best || isBetterScore([matched, filled], bestScore, method, best)) {
      best = method
      bestScore = [matched, filled]
    }
  }
  if (best)
    return best.name

  return (methods.find(method => method.recommended) ?? methods[0]).name
}

/**
 * Keeps only the selected method's credentials in the map. Keys that belong
 * solely to other methods move into the stash, so switching back restores
 * them while a save never carries a stale token of another method. Fixed
 * values of the selected method are written, those of other methods removed.
 * A fixed value may target a field of another method; the typed value then
 * waits in the stash until that method is selected again.
 */
export function applyMethod(
  credentials: Record<string, string>,
  form: DNSProviderForm | undefined,
  methodName: string | undefined,
  stash: MethodStash,
): void {
  const methods = formMethods(form)
  const method = methods.find(item => item.name === methodName)
  if (!method)
    return

  const own = new Set(method.fields ?? [])
  const ownValues = method.values ?? {}
  const listed = new Set(methods.flatMap(item => item.fields ?? []))
  const isFixed = (key: string, value: string | undefined) =>
    methods.some(item => item.values?.[key] !== undefined && item.values[key] === value)

  for (const key of listed) {
    if (own.has(key)) {
      // A fixed value left by another method is not the user's input.
      if (key in credentials && isFixed(key, credentials[key]))
        delete credentials[key]
      if (!(key in credentials) && key in stash)
        credentials[key] = stash[key]
      delete stash[key]
      continue
    }
    if (key in credentials) {
      if (hasValue(credentials[key]) && !isFixed(key, credentials[key]))
        stash[key] = credentials[key]
      delete credentials[key]
    }
  }

  for (const other of methods) {
    for (const key of Object.keys(other.values ?? {})) {
      if (!listed.has(key) && !(key in ownValues))
        delete credentials[key]
    }
  }
  Object.assign(credentials, ownValues)
}

/** Map a field value is stored in, decided by its group. */
export function configurationTarget(field: DNSProviderField): ConfigurationTarget {
  return field.group === 'setting' ? 'additional' : 'credentials'
}

/** Number of settings holding a value other than their default. */
export function changedSettingsCount(fields: DNSProviderField[], values: (field: DNSProviderField) => string | undefined): number {
  return fields.filter(field => {
    const value = values(field)
    return hasValue(value) && String(value).trim() !== (field.default ?? '')
  }).length
}
