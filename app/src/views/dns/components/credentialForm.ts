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

/**
 * Method to preselect: when editing, the one whose keys hold the most values,
 * otherwise the recommended one, otherwise the first.
 */
export function initialMethod(form: DNSProviderForm | undefined, credentials: Record<string, string> | undefined): string | undefined {
  const methods = formMethods(form)
  if (methods.length === 0)
    return undefined

  let best: DNSProviderMethod | undefined
  let bestScore = 0
  for (const method of methods) {
    const score = (method.fields ?? []).filter(key => hasValue(credentials?.[key])).length
    if (score > bestScore || (score === bestScore && score > 0 && method.recommended && !best?.recommended)) {
      best = method
      bestScore = score
    }
  }
  if (best)
    return best.name

  return (methods.find(method => method.recommended) ?? methods[0]).name
}

/**
 * Keeps only the selected method's credentials in the map. Keys that belong
 * solely to other methods move into the stash, so switching back restores
 * them while a save never carries a stale token of another method.
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
  const listed = new Set(methods.flatMap(item => item.fields ?? []))
  for (const key of listed) {
    if (own.has(key)) {
      if (!(key in credentials) && key in stash)
        credentials[key] = stash[key]
      delete stash[key]
      continue
    }
    if (key in credentials) {
      if (hasValue(credentials[key]))
        stash[key] = credentials[key]
      delete credentials[key]
    }
  }
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
