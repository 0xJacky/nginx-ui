/** A DNS credential created through `openDnsCredentialEditor`. */
export interface CreatedDnsCredential {
  id: number
  name: string
  code: string
  provider?: string
  provider_code?: string
}

interface EditorRequest {
  id: number
  resolve: (credential: CreatedDnsCredential | undefined) => void
}

let nextRequestId = 0

/** Open requests, rendered one at a time by `DnsCredentialEditorHost`. */
export const dnsCredentialEditorRequests = shallowRef<EditorRequest[]>([])

/** Settles a request and removes it from the queue. */
export function settleDnsCredentialEditor(id: number, credential?: CreatedDnsCredential) {
  const request = dnsCredentialEditorRequests.value.find(item => item.id === id)
  if (!request)
    return

  dnsCredentialEditorRequests.value = dnsCredentialEditorRequests.value.filter(item => item.id !== id)
  request.resolve(credential)
}

/**
 * Opens the DNS credential form in a modal above the current page, so a
 * credential can be added without leaving a half filled form.
 *
 * Resolves with the created credential, or with undefined when the user
 * cancels. The modal is rendered by `DnsCredentialEditorHost` inside the app
 * root, so it shares the theme, locale, router and stores of the page.
 * Plugins reach it as `window.NginxUI.shared.ui.openDnsCredentialEditor`.
 */
export function openDnsCredentialEditor(): Promise<CreatedDnsCredential | undefined> {
  return new Promise(resolve => {
    dnsCredentialEditorRequests.value = [
      ...dnsCredentialEditorRequests.value,
      { id: ++nextRequestId, resolve },
    ]
  })
}
