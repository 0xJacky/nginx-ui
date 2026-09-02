import type { HostDiagnosis, NginxDiscovery, SetupParams } from '@/api/host_setup'

/**
 * Values the target host actually reported. Only these take part in the
 * detected against overridden comparison, so a field the server never reports
 * can never mark a user supplied value as stale.
 */
export function detectedSettings(diagnosis: HostDiagnosis): Partial<SetupParams> {
  const detected: Partial<SetupParams> = {}
  if (diagnosis.service_manager)
    detected.service_manager = diagnosis.service_manager
  if (diagnosis.systemctl_path)
    detected.systemctl_path = diagnosis.systemctl_path
  if (diagnosis.launchctl_path)
    detected.launchctl_path = diagnosis.launchctl_path
  if (diagnosis.systemd_unit)
    detected.systemd_unit = diagnosis.systemd_unit
  if (diagnosis.launchd_service)
    detected.launchd_service = diagnosis.launchd_service

  return { ...detected, ...discoveredNginxPaths(diagnosis.nginx) }
}

/** nginx paths compiled into the binary that answered `nginx -V`. */
export function discoveredNginxPaths(nginx: NginxDiscovery | null | undefined): Partial<SetupParams> {
  const detected: Partial<SetupParams> = {}
  if (!nginx)
    return detected
  if (nginx.executable_path)
    detected.nginx_sbin_path = nginx.executable_path
  if (nginx.config_dir)
    detected.host_config_dir = nginx.config_dir
  if (nginx.log_dir)
    detected.host_log_dir = nginx.log_dir
  if (nginx.pid_path)
    detected.pid_path = nginx.pid_path
  return detected
}

/**
 * Conventional values used only when detection did not report one. They fill
 * blanks and never overwrite an existing value.
 */
export function suggestedDefaults(diagnosis: HostDiagnosis): Partial<SetupParams> {
  const suggested: Partial<SetupParams> = {}
  if (diagnosis.service_manager === 'launchd') {
    suggested.launchd_service = 'homebrew.mxcl.nginx'
    const prefix = diagnosis.homebrew_prefix
    if (prefix) {
      suggested.nginx_sbin_path = `${prefix}/opt/nginx/bin/nginx`
      suggested.host_config_dir = `${prefix}/etc/nginx`
      suggested.host_log_dir = `${prefix}/var/log/nginx`
      suggested.pid_path = `${prefix}/var/run/nginx.pid`
    }
  }
  else if (diagnosis.service_manager === 'systemd') {
    suggested.systemd_unit = 'nginx.service'
  }
  return suggested
}

/** True when at least one reported value differs from the current params. */
export function hasDiagnosisChanges(params: SetupParams, diagnosis: HostDiagnosis) {
  return Object.entries(detectedSettings(diagnosis)).some(([key, value]) => {
    return value !== undefined && params[key as keyof SetupParams] !== value
  })
}

/** Apply reported values, then fill any still-empty field with a suggestion. */
export function applyHostDiagnosis(params: SetupParams, diagnosis: HostDiagnosis) {
  Object.assign(params, detectedSettings(diagnosis))
  for (const [key, value] of Object.entries(suggestedDefaults(diagnosis))) {
    const field = key as keyof SetupParams
    if (!params[field])
      Object.assign(params, { [field]: value })
  }
}
