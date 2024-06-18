import type { customRender } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'

export const detailRender = (args: customRender) => {
  switch (args.record.title) {
    case 'Sync Certificate Success':
      return syncCertificateSuccess(args.text)
    case 'Sync Certificate Error':
      return syncCertificateError(args.text)
    default:
      return args.text
  }
}

function syncCertificateSuccess(text: string) {
  const data = JSON.parse(text)

  return $gettext('Sync Certificate %{cert_name} to %{env_name} successfully',
    { cert_name: data.cert_name, env_name: data.env_name })
}

function syncCertificateError(text: string) {
  const data = JSON.parse(text)

  if (data.status_code === 404) {
    return $gettext('Sync Certificate %{cert_name} to %{env_name} failed, please upgrade the remote Nginx UI to the latest version',
      { cert_name: data.cert_name, env_name: data.env_name }, true)
  }

  return $gettext('Sync Certificate %{cert_name} to %{env_name} failed, response: %{resp}',
    { cert_name: data.cert_name, env_name: data.env_name, resp: data.resp_body }, true)
}
