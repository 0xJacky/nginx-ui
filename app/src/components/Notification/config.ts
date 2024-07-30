export function syncConfigSuccess(text: string) {
  const data = JSON.parse(text)

  return $gettext('Sync Config %{config_name} to %{env_name} successfully',
    { config_name: data.config_name, env_name: data.env_name })
}

export function syncConfigError(text: string) {
  const data = JSON.parse(text)

  if (data.status_code === 404) {
    return $gettext('Sync config %{config_name} to %{env_name} failed, please upgrade the remote Nginx UI to the latest version',
      { config_name: data.config_name, env_name: data.env_name }, true)
  }

  return $gettext('Sync config %{config_name} to %{env_name} failed, response: %{resp}',
    { config_name: data.cert_name, env_name: data.env_name, resp: data.resp_body }, true)
}

export function syncRenameConfigSuccess(text: string) {
  const data = JSON.parse(text)

  return $gettext('Rename %{orig_path} to %{new_path} on %{env_name} successfully',
    { orig_path: data.orig_path, new_path: data.orig_path, env_name: data.env_name })
}

export function syncRenameConfigError(text: string) {
  const data = JSON.parse(text)

  if (data.status_code === 404) {
    return $gettext('Rename %{orig_path} to %{new_path} on %{env_name} failed, please upgrade the remote Nginx UI to the latest version',
      { orig_path: data.orig_path, new_path: data.orig_path, env_name: data.env_name }, true)
  }

  return $gettext('Rename %{orig_path} to %{new_path} on %{env_name} failed, response: %{resp}',
    { orig_path: data.orig_path, new_path: data.orig_path, resp: data.resp_body, env_name: data.env_name }, true)
}
