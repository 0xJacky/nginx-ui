// Display names for the enum values of the ingress API. Unknown values are
// shown as they come, so a newer backend never renders an empty cell.

const phaseLabels: Record<string, () => string> = {
  Pending: () => $gettext('Pending'),
  Applying: () => $gettext('Applying'),
  Ready: () => $gettext('Ready'),
  Degraded: () => $gettext('Degraded'),
  Error: () => $gettext('Error'),
}

const conditionLabels: Record<string, () => string> = {
  ConfigApplied: () => $gettext('Config Applied'),
  CertReady: () => $gettext('Certificate Ready'),
  BackendsHealthy: () => $gettext('Backends Healthy'),
}

const tlsModeLabels: Record<string, () => string> = {
  none: () => $gettext('HTTP only'),
  acme: () => $gettext('Issued automatically (ACME)'),
  cert: () => $gettext('Existing certificate'),
}

const challengeLabels: Record<string, () => string> = {
  http01: () => $gettext('HTTP-01'),
  dns01: () => $gettext('DNS-01'),
}

const lbLabels: Record<string, () => string> = {
  round_robin: () => $gettext('Round Robin'),
  least_conn: () => $gettext('Least Connections'),
  ip_hash: () => $gettext('IP Hash'),
}

function lookup(labels: Record<string, () => string>, value?: string) {
  if (!value)
    return ''
  return labels[value]?.() ?? value
}

export function phaseLabel(phase?: string) {
  return lookup(phaseLabels, phase || 'Pending')
}

export function conditionLabel(type?: string) {
  return lookup(conditionLabels, type)
}

export function tlsModeLabel(mode?: string) {
  return lookup(tlsModeLabels, mode || 'none')
}

export function challengeLabel(challenge?: string) {
  return lookup(challengeLabels, challenge || 'http01')
}

export function lbLabel(lb?: string) {
  return lookup(lbLabels, lb || 'round_robin')
}
