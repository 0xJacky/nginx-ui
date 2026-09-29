import type { Cert, CertRenewalMethod, CertState } from '@/api/cert'
import { AutoCertState } from '@/constants'

/** Color family of a state, mapped to antdv-next status colors. */
export type CertStateTone = 'success' | 'warning' | 'error' | 'processing' | 'default'

export function certStateTone(state?: CertState): CertStateTone {
  switch (state) {
    case 'valid':
      return 'success'
    case 'expiring':
      return 'warning'
    case 'expired':
    case 'failed':
      return 'error'
    case 'issuing':
      return 'processing'
    default:
      return 'default'
  }
}

/** Human readable state, e.g. "Valid · 62 days left". */
export function certStateLabel(cert: Pick<Cert, 'state' | 'days_left' | 'certificate_info'>): string {
  const days = String(Math.max(cert.days_left ?? 0, 0))
  switch (cert.state) {
    case 'valid':
      return $gettext('Valid · %{days} days left', { days })
    case 'expiring':
      return $gettext('Expires in %{days} days', { days })
    case 'expired':
      return $gettext('Expired')
    case 'failed':
      return cert.certificate_info ? $gettext('Renewal failed') : $gettext('Issuance failed')
    case 'issuing':
      return $gettext('Issuing...')
    case 'manual':
      return $gettext('Managed manually')
    default:
      return $gettext('Not issued yet')
  }
}

/** Renewal method as shown in the list, e.g. "DNS validation · Cloudflare". */
export function certRenewalMethodLabel(method?: CertRenewalMethod, provider?: string): string {
  switch (method) {
    case 'dns01':
      return provider
        ? $gettext('DNS validation · %{provider}', { provider })
        : $gettext('DNS validation')
    case 'http01':
      return $gettext('HTTP validation')
    case 'self_signed':
      return $gettext('Self-signed')
    case 'sync':
      return $gettext('Synced from another node')
    default:
      return $gettext('Upload')
  }
}

/** Certificates issued through ACME by Nginx UI, whatever their renewal state. */
export function isAcmeCert(cert: Pick<Cert, 'auto_cert' | 'challenge_method'>): boolean {
  if (cert.auto_cert === AutoCertState.Sync || cert.auto_cert === AutoCertState.SelfSigned)
    return false
  return !!cert.challenge_method
}

/** Shows at most `limit` items and the number of the rest. */
export function splitVisible<T>(items: T[], limit: number): { visible: T[], hidden: number } {
  return {
    visible: items.slice(0, limit),
    hidden: Math.max(items.length - limit, 0),
  }
}
