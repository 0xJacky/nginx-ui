import type { HTTPSOnboardingPhase } from '../site_edit/components/HTTPS/httpsOnboardingState'
import type { HTTPSCardMethod } from '../site_edit/components/HTTPS/httpsRequest'
import type { NgxConfig } from '@/api/ngx'
import { hasPendingTLSServer, hasTLSServer } from '../site_edit/components/HTTPS/siteHTTPSState'

// Pure decisions for the Add Site wizard's mode selector and SSL step. Both
// modes share one SSL step: the HTTPS card is the only way to set up HTTPS and
// the configuration editor sits below it in a collapse.

export type WizardMode = 'quick' | 'advanced'

export const SSL_STEP = 2

export const EDITOR_PANEL_KEY = 'editor'

/**
 * The mode decides how step 0 builds the configuration, so it can only change
 * there. Later steps keep the selector visible but locked; switching would
 * otherwise flip a draft that is already built (and maybe saved).
 */
export function isModeLocked(step: number): boolean {
  return step > 0
}

/** The "Edit configuration file" panel starts collapsed in Quick Setup and expanded in Advanced. */
export function defaultEditorPanelKeys(mode: WizardMode): string[] {
  return mode === 'advanced' ? [EDITOR_PANEL_KEY] : []
}

/**
 * - `pending`: a TLS server waits for its certificate (e.g. Quick Setup with TLS).
 * - `configured`: every TLS server already has a certificate (hand-written in Advanced).
 * - `none`: plain HTTP; the HTTPS run generates the TLS server from the port-80 server.
 */
export type SSLStepTLSState = 'none' | 'pending' | 'configured'

export function sslStepTLSState(config: Pick<NgxConfig, 'servers'> | undefined | null): SSLStepTLSState {
  if (hasPendingTLSServer(config))
    return 'pending'
  return hasTLSServer(config) ? 'configured' : 'none'
}

/**
 * Label of the wizard's Finish button on the SSL step. It runs what the HTTPS
 * card is set to, so the label names the HTTPS work it starts.
 */
export function sslStepFinishLabel(method: HTTPSCardMethod | undefined, phase: HTTPSOnboardingPhase | undefined): string {
  if (!method || method === 'skip')
    return $gettext('Finish')
  if (phase === 'error')
    return $gettext('Retry')
  return method === 'existing'
    ? $gettext('Enable HTTPS and finish')
    : $gettext('Issue certificate and finish')
}
