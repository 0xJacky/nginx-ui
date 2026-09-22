import type { ModelBase } from '@/api/curd'
import type { ConfigurationField } from '@/api/plugin'
import { http, useCurdApi } from '@uozi-admin/request'

/** An external target certificates are pushed to, served by a cert.deploy plugin. */
export interface CertDeployTarget extends ModelBase {
  name: string
  /** "plugin:<code>", the target kind of a cert.deploy plugin. */
  kind: string
  config: Record<string, string>
  /** The bound certificate, 0 binds every certificate. */
  cert_id: number
  enabled: boolean
}

/** A kind of target an enabled cert.deploy plugin offers, with its form. */
export interface DeployTargetKind {
  kind: string
  name: string
  plugin_id?: string
  fields: ConfigurationField[]
}

export type CertDeploymentStatus = 'ok' | 'failed'

/** The outcome of pushing one certificate to one target. */
export interface CertDeployment {
  id: number
  target_id: number
  cert_id: number
  status: CertDeploymentStatus
  message: string
  attempts: number
  created_at: string
}

/** The outcome of a push someone asked for. */
export interface DeployResult {
  target_id: number
  cert_id: number
  status: CertDeploymentStatus
  message: string
}

/** A target bound to a certificate with its latest outcome for it. */
export interface CertificateDeployTarget {
  target: Omit<CertDeployTarget, 'config'>
  /** Empty when no enabled plugin offers the kind right now. */
  kind_name?: string
  last?: CertDeployment
}

const certDeployTarget = useCurdApi<CertDeployTarget>('/cert_deploy_targets')

/** Lists the target kinds enabled cert.deploy plugins offer. */
export function listDeployKinds(): Promise<{ data: DeployTargetKind[] }> {
  return http.get('/cert_deploy_targets/kinds')
}

/** Lists the latest outcomes of a target, newest first. */
export function listDeployments(targetId: number, certId?: number): Promise<{ data: CertDeployment[] }> {
  return http.get(`/cert_deploy_targets/${targetId}/deployments`, { params: certId ? { cert_id: certId } : undefined })
}

/** Pushes every certificate bound to a target once. */
export function deployTarget(targetId: number): Promise<{ data: DeployResult[] }> {
  return http.post(`/cert_deploy_targets/${targetId}/deploy`)
}

/** Dry run of a target configuration against a certificate, 0 picks one. */
export function testDeployTarget(params: { kind: string, config: Record<string, string>, cert_id: number }): Promise<{ message: string }> {
  return http.post('/cert_deploy_targets/test', params)
}

/** Pushes a certificate to every enabled target bound to it once. */
export function deployCertificate(certId: number): Promise<{ data: DeployResult[] }> {
  return http.post(`/certs/${certId}/deploy`)
}

/** Lists the targets bound to a certificate with their latest outcome. */
export function listCertificateDeployTargets(certId: number): Promise<{ data: CertificateDeployTarget[] }> {
  return http.get(`/certs/${certId}/deploy_targets`)
}

export default certDeployTarget
