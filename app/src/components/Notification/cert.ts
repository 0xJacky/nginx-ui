export function syncCertificateSuccess(text: string) {
  const data = JSON.parse(text)

  return $gettext('Sync Certificate %{cert_name} to %{env_name} successfully', { cert_name: data.cert_name, env_name: data.env_name })
}

export function syncCertificateError(text: string) {
  const data = JSON.parse(text)

  if (data.status_code === 404) {
    return $gettext('Sync Certificate %{cert_name} to %{env_name} failed, please upgrade the remote Nginx UI to the latest version', { cert_name: data.cert_name, env_name: data.env_name }, true)
  }

  return $gettext('Sync Certificate %{cert_name} to %{env_name} failed, response: %{resp}', { cert_name: data.cert_name, env_name: data.env_name, resp: data.resp_body }, true)
}
