import type { CheckGroup, StepOutcome, VerifyResult } from '@/api/host_setup'

export type { CheckGroup }

export type CheckLevel = 'success' | 'warning' | 'error'

export interface CheckRow {
  key: string
  label: string
  outcome: StepOutcome
  level: CheckLevel
}

// Backend check identifiers, mapped so the result list is readable.
const checkLabels: Record<string, () => string> = {
  config_dir_shared: () => $gettext('nginx config directory is the host directory'),
  config_dir_writable: () => $gettext('nginx config directory is writable'),
  host_platform: () => $gettext('Host platform matches the service manager'),
  known_hosts_persistence: () => $gettext('known_hosts is persisted'),
  launchctl_service_loaded: () => $gettext('launchd service is loaded'),
  log_dir_readable: () => $gettext('nginx log directory is readable'),
  nginx_test: () => $gettext('nginx -t passes'),
  pid_file_present: () => $gettext('nginx PID file is present'),
  platform: () => $gettext('Host platform'),
  same_host: () => $gettext('Target is the container host'),
  ssh_connect: () => $gettext('SSH connection'),
  sudo_available: () => $gettext('Passwordless sudo is available'),
  sudoers_coverage: () => $gettext('sudoers rules cover the nginx commands'),
  systemctl_is_active: () => $gettext('systemd unit is active'),
  unit_has_execreload: () => $gettext('systemd unit defines ExecReload'),
}

export function checkLabel(key: string) {
  return checkLabels[key]?.() ?? key
}

export function checkLevel(outcome: StepOutcome): CheckLevel {
  if (outcome.level)
    return outcome.level
  return outcome.ok ? 'success' : 'error'
}

export function toCheckRows(result: VerifyResult | null): CheckRow[] {
  return Object.entries(result?.steps ?? {}).map(([key, outcome]) => ({
    key,
    label: checkLabel(key),
    outcome,
    level: checkLevel(outcome),
  }))
}

export function tagColor(level: CheckLevel) {
  switch (level) {
    case 'success':
      return 'success'
    case 'warning':
      return 'warning'
    default:
      return 'error'
  }
}

export function tagText(level: CheckLevel) {
  switch (level) {
    case 'success':
      return $gettext('OK')
    case 'warning':
      return $gettext('Warning')
    default:
      return $gettext('Failed')
  }
}

/** Warnings do not block, so only hard failures gate the wizard. */
export function hasBlockingFailure(rows: CheckRow[]) {
  return rows.some(row => row.level === 'error')
}
