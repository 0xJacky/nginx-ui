import type { NginxControlMode, NginxControlSettings, NginxSettings } from '@/api/settings'

export function resolveNginxControlMode(nginx: NginxSettings): NginxControlMode {
  if (nginx.host_mode === 'ssh')
    return 'host_via_ssh'
  if (nginx.container_name)
    return 'external_container'
  return 'local'
}

export function cloneNginxSettings(nginx: NginxSettings): NginxSettings {
  return {
    ...nginx,
    log_dir_white_list: [...(nginx.log_dir_white_list ?? [])],
  }
}

export function buildNginxControlPayload(
  nginx: NginxSettings,
  mode: NginxControlMode,
  containerName = '',
): NginxControlSettings {
  return {
    mode,
    container_name: mode === 'external_container' ? containerName.trim() : '',
    host_address: nginx.host_address,
    host_user: nginx.host_user,
    host_access_mode: nginx.host_access_mode,
    host_auth_method: nginx.host_auth_method,
    host_key_source: nginx.host_key_source,
    host_private_key_path: nginx.host_private_key_path,
    host_password_ref: nginx.host_password_ref,
    host_known_hosts_path: nginx.host_known_hosts_path,
    host_sudo_prefix: nginx.host_sudo_prefix,
    host_service_manager: nginx.host_service_manager,
    host_systemd_unit_name: nginx.host_systemd_unit_name,
    host_systemctl_path: nginx.host_systemctl_path,
    host_launchd_service: nginx.host_launchd_service,
    host_launchctl_path: nginx.host_launchctl_path,
    host_config_dir: nginx.host_config_dir,
    host_log_dir: nginx.host_log_dir,
    sbin_path: nginx.sbin_path,
    pid_path: nginx.pid_path,
    config_dir: nginx.config_dir,
    config_path: nginx.config_path,
    access_log_path: nginx.access_log_path,
    error_log_path: nginx.error_log_path,
  }
}

export function applyNginxControlSettings(target: NginxSettings, settings: NginxControlSettings) {
  Object.assign(target, {
    container_name: settings.container_name,
    host_mode: settings.mode === 'host_via_ssh' ? 'ssh' : '',
    host_address: settings.host_address,
    host_user: settings.host_user,
    host_access_mode: settings.host_access_mode,
    host_auth_method: settings.host_auth_method,
    host_key_source: settings.host_key_source,
    host_private_key_path: settings.host_private_key_path,
    host_password_ref: settings.host_password_ref,
    host_known_hosts_path: settings.host_known_hosts_path,
    host_sudo_prefix: settings.host_sudo_prefix,
    host_service_manager: settings.host_service_manager,
    host_systemd_unit_name: settings.host_systemd_unit_name,
    host_systemctl_path: settings.host_systemctl_path,
    host_launchd_service: settings.host_launchd_service,
    host_launchctl_path: settings.host_launchctl_path,
    host_config_dir: settings.host_config_dir,
    host_log_dir: settings.host_log_dir,
    sbin_path: settings.sbin_path,
    pid_path: settings.pid_path,
    config_dir: settings.config_dir,
    config_path: settings.config_path,
    access_log_path: settings.access_log_path,
    error_log_path: settings.error_log_path,
  })
}
