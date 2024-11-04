export function syncConfigSuccess(text: string) {
  const data = JSON.parse(text)

  return $gettext('Sync Config %{config_name} to %{env_name} successfully', { config_name: data.config_name, env_name: data.env_name })
}

export function syncConfigError(text: string) {
  const data = JSON.parse(text)

  if (data.status_code === 404) {
    return $gettext('Please upgrade the remote Nginx UI to the latest version')
  }

  return $gettext('Sync config %{config_name} to %{env_name} failed, response: %{resp}', { config_name: data.config_name, env_name: data.env_name, resp: data.resp_body }, true)
}

export function syncRenameConfigSuccess(text: string) {
  const data = JSON.parse(text)

  return $gettext('Rename %{orig_path} to %{new_path} on %{env_name} successfully', { orig_path: data.orig_path, new_path: data.new_path, env_name: data.env_name })
}

export function syncRenameConfigError(text: string) {
  const data = JSON.parse(text)

  if (data.status_code === 404) {
    return $gettext('Please upgrade the remote Nginx UI to the latest version')
  }

  return $gettext('Rename %{orig_path} to %{new_path} on %{env_name} failed, response: %{resp}', { orig_path: data.orig_path, new_path: data.new_path, resp: data.resp_body, env_name: data.env_name }, true)
}

export function saveSiteSuccess(text: string) {
  const data = JSON.parse(text)
  return $gettext('Save Site %{site} to %{node} successfully', { site: data.site, node: data.node })
}

export function saveSiteError(text: string) {
  const data = JSON.parse(text)
  if (data.status_code === 404) {
    return $gettext('Please upgrade the remote Nginx UI to the latest version')
  }
  return $gettext('Save site %{site} to %{node} error, response: %{resp}', { site: data.name, node: data.node, resp: JSON.stringify(data.response) }, true)
}

export function deleteSiteSuccess(text: string) {
  const data = JSON.parse(text)
  return $gettext('Remove Site %{site} from %{node} successfully', { site: data.name, node: data.node })
}

export function deleteSiteError(text: string) {
  const data = JSON.parse(text)
  if (data.status_code === 404) {
    return $gettext('Please upgrade the remote Nginx UI to the latest version')
  }
  return $gettext('Remove site %{site} from %{node} error, response: %{resp}', { site: data.name, node: data.node, resp: JSON.stringify(data.response) }, true)
}

export function enableSiteSuccess(text: string) {
  const data = JSON.parse(text)
  return $gettext('Enable Site %{site} on %{node} successfully', { site: data.name, node: data.node })
}

export function enableSiteError(text: string) {
  const data = JSON.parse(text)
  if (data.status_code === 404) {
    return $gettext('Please upgrade the remote Nginx UI to the latest version')
  }
  return $gettext('Enable site %{site} on %{node} error, response: %{resp}', { site: data.name, node: data.node, resp: JSON.stringify(data.response) }, true)
}

export function disableSiteSuccess(text: string) {
  const data = JSON.parse(text)
  return $gettext('Disable Site %{site} on %{node} successfully', { site: data.name, node: data.node })
}

export function disableSiteError(text: string) {
  const data = JSON.parse(text)
  if (data.status_code === 404) {
    return $gettext('Please upgrade the remote Nginx UI to the latest version')
  }
  return $gettext('Disable site %{site} on %{node} error, response: %{resp}', { site: data.name, node: data.node, resp: JSON.stringify(data.response) }, true)
}

export function renameSiteSuccess(text: string) {
  const data = JSON.parse(text)
  return $gettext('Rename Site %{site} to %{new_site} on %{node} successfully', { site: data.name, new_site: data.new_name, node: data.node })
}

export function renameSiteError(text: string) {
  const data = JSON.parse(text)
  if (data.status_code === 404) {
    return $gettext('Please upgrade the remote Nginx UI to the latest version')
  }
  return $gettext('Rename Site %{site} to %{new_site} on %{node} error, response: %{resp}', { site: data.name, new_site: data.new_name, node: data.node, resp: JSON.stringify(data.response) }, true)
}
