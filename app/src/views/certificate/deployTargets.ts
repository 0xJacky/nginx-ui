import type { DeployResult, DeployTargetKind } from '@/api/cert_deploy'
import cert from '@/api/cert'
import { listDeployKinds } from '@/api/cert_deploy'

// Target kinds enabled cert.deploy plugins offer, with the form each declares.
export const deployKinds = ref<DeployTargetKind[]>([])

let pendingKinds: Promise<void> | undefined

/** Loads the target kinds once; pass force to refresh them. */
export function loadDeployKinds(force = false): Promise<void> {
  if (pendingKinds && !force)
    return pendingKinds

  pendingKinds = listDeployKinds()
    .then(res => {
      deployKinds.value = res.data ?? []
    })
    .catch(error => {
      console.error('Failed to load deploy target kinds:', error)
      pendingKinds = undefined
    })
  return pendingKinds
}

export function findDeployKind(kind?: string): DeployTargetKind | undefined {
  if (!kind)
    return undefined
  return deployKinds.value.find(item => item.kind === kind)
}

/** Display name of a kind, falling back to the stored value. */
export function deployKindLabel(kind?: string): string {
  return findDeployKind(kind)?.name ?? kind ?? ''
}

/** Drops the values the kind form does not declare. */
export function sanitizeDeployConfig(kind: string | undefined, config: Record<string, string> | undefined): Record<string, string> {
  const found = findDeployKind(kind)
  if (!found)
    return { ...config }
  const allowed = new Set(found.fields.map(field => field.key))
  return Object.fromEntries(Object.entries(config ?? {}).filter(([key]) => allowed.has(key)))
}

export interface CertificateOption {
  id: number
  name: string
}

// Every certificate, for the binding selector and for showing names.
export const certificateOptions = ref<CertificateOption[]>([])

let pendingCertificates: Promise<void> | undefined

/** Loads every certificate page by page, once; pass force to refresh. */
export function loadCertificateOptions(force = false): Promise<void> {
  if (pendingCertificates && !force)
    return pendingCertificates

  pendingCertificates = (async () => {
    const list: CertificateOption[] = []
    let page = 1
    while (true) {
      const res = await cert.getList({ page })
      const data = res?.data ?? []
      list.push(...data.map(item => ({ id: item.id, name: item.name || item.filename || `#${item.id}` })))
      const perPage = res?.pagination?.per_page ?? 0
      if (!perPage || data.length < perPage)
        break
      page++
    }
    certificateOptions.value = list
  })().catch(error => {
    console.error('Failed to load certificates:', error)
    pendingCertificates = undefined
  })
  return pendingCertificates
}

/** Name of a bound certificate, or "All certificates" for 0. */
export function certificateLabel(certId?: number): string {
  if (!certId)
    return $gettext('All certificates')
  return certificateOptions.value.find(item => item.id === certId)?.name ?? `#${certId}`
}

/** One line that sums up the results of a push someone asked for. */
export function summarizeResults(results: DeployResult[]): { ok: number, failed: number } {
  const failed = results.filter(result => result.status === 'failed').length
  return { ok: results.length - failed, failed }
}
