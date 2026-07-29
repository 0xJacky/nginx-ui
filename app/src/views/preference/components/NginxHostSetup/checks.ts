import type { CheckGroup, RemediationCode, StepOutcome, StepRemediation, VerifyResult } from '@/api/host_setup'

export type { CheckGroup }

export type CheckLevel = 'success' | 'warning' | 'error'

export interface CheckRow {
  key: string
  label: string
  outcome: StepOutcome
  suggestedFix?: string
  suggestedCommand?: string
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

const remediationMessages: Record<RemediationCode, (params: Record<string, string>) => string> = {
  add_bind_mount: params => $gettext('Add a bind mount: -v %{source}:%{target}', {
    source: params.source,
    target: params.target,
  }),
  add_missing_sudoers_entries: params => $gettext('Append the missing entries to %{path} (see the Install step).', {
    path: params.path,
  }),
  check_host_address: () => $gettext('Confirm that the address reaches the machine running nginx.'),
  check_host_address_outside_docker: () => $gettext('This is expected on Docker Desktop. On other runtimes, confirm that the address reaches the machine running nginx.'),
  check_ssh_connection: () => $gettext('Confirm that the SSH server is running, the user exists, and the key or password is correct.'),
  check_systemd_unit: () => $gettext('Confirm that the systemd unit name matches the installation, such as nginx.service or openresty.service.'),
  confirm_homebrew_service_owner: () => $gettext('Confirm that the configured SSH user owns the Homebrew service.'),
  confirm_host_nginx_directory: () => $gettext('Confirm that the configured nginx directory exists on the host.'),
  confirm_macos_host_alias: () => $gettext('Confirm that the container runtime provides host.docker.internal.'),
  confirm_uname: () => $gettext('Confirm that /usr/bin/uname exists on the host.'),
  correct_parameters: () => $gettext('Correct the highlighted path or identifier, then run verification again.'),
  fix_nginx_config: () => $gettext('Fix the nginx configuration error shown in the details.'),
  inspect_sudo_permissions: () => $gettext('Run `sudo -l` on the host to inspect the configured permissions.'),
  inspect_systemd_unit: () => $gettext('Inspect the systemd unit file.'),
  mount_nginx_logs: () => $gettext('Bind-mount the host nginx log directory read-only at the configured container log path, and ensure the container can read it.'),
  mount_pid_directory: params => $gettext('Bind-mount the host PID directory containing %{path} at the same container path.', {
    path: params.path,
  }),
  persist_known_hosts: () => $gettext('Persist /etc/nginx-ui with a Docker named volume or bind mount so known_hosts survives container rebuilds.'),
  replace_bind_mount: params => $gettext('Replace the mount with: -v %{source}:%{target}', {
    source: params.source,
    target: params.target,
  }),
  restart_without_exec_reload: () => $gettext('Some packages omit ExecReload. Use `systemctl restart` instead.'),
  review_cross_host_guide: () => $gettext('If this is a remote host, review the cluster Node cross-host guide.'),
  review_install_permissions: () => $gettext('Review the file permission commands in the Install step.'),
  review_sudoers_rules: () => $gettext('Review the sudoers rules shown in the Install step.'),
  select_service_manager: () => $gettext('Select the service manager that matches the SSH host.'),
  start_homebrew_nginx: () => $gettext('Run `brew services start nginx` as the configured SSH user.'),
  use_cluster_node: () => $gettext('Review the cluster Node cross-host guide for the correct deployment method.'),
  use_docker_host_alias: () => $gettext('Use host.docker.internal when nginx runs on the Docker host.'),
  use_macos_host_alias: () => $gettext('Use host.docker.internal when nginx runs on the macOS host.'),
  verify_bind_mount: () => $gettext('Verify the bind mount manually.'),
}

const remediationCommands: Partial<Record<RemediationCode, string>> = {
  inspect_sudo_permissions: 'sudo -l',
  start_homebrew_nginx: 'brew services start nginx',
}

export function checkLabel(key: string) {
  return checkLabels[key]?.() ?? key
}

export function remediationText(remediation: StepRemediation) {
  return remediationMessages[remediation.code](remediation.params)
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
    suggestedFix: outcome.remediation ? remediationText(outcome.remediation) : undefined,
    suggestedCommand: outcome.remediation ? remediationCommands[outcome.remediation.code] : undefined,
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
