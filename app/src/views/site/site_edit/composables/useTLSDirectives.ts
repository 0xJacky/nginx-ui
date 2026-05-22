import { storeToRefs } from 'pinia'
import { useSiteEditorStore } from '../components/SiteEditor/store'

// useTLSDirectives provides helpers that write SSL directives into the
// currently edited server block.
export function useTLSDirectives() {
  const editorStore = useSiteEditorStore()
  const { curServerDirectives, curDirectivesMap } = storeToRefs(editorStore)

  function hasTLSListen(params: string) {
    return params.includes('443') && params.includes('ssl')
  }

  function ensureDirective(directive: string, params: string, insertIndex?: number) {
    if (!curServerDirectives.value)
      curServerDirectives.value = []

    const existingDirective = curServerDirectives.value.find(v => v.directive === directive)
    if (existingDirective) {
      existingDirective.params = params
      return
    }

    const directiveItem = { directive, params }
    if (insertIndex === undefined || insertIndex < 0 || insertIndex > curServerDirectives.value.length) {
      curServerDirectives.value.push(directiveItem)
      return
    }
    curServerDirectives.value.splice(insertIndex, 0, directiveItem)
  }

  function ensureTLSDirectives(sslCertificate: string, sslCertificateKey: string) {
    if (!curServerDirectives.value)
      curServerDirectives.value = []

    const hasIPv4TLSListen = curServerDirectives.value.some(v => v.directive === 'listen' && hasTLSListen(v.params) && !v.params.includes('[::]'))
    const hasIPv6TLSListen = curServerDirectives.value.some(v => v.directive === 'listen' && hasTLSListen(v.params) && v.params.includes('[::]'))

    if (!hasIPv6TLSListen) {
      curServerDirectives.value.splice(0, 0, {
        directive: 'listen',
        params: '[::]:443 ssl',
      })
    }
    if (!hasIPv4TLSListen) {
      curServerDirectives.value.splice(0, 0, {
        directive: 'listen',
        params: '443 ssl',
      })
    }

    const serverNameIdx = curDirectivesMap.value.server_name?.[0]?.idx ?? (curServerDirectives.value.length - 1)
    ensureDirective('ssl_certificate', sslCertificate, serverNameIdx + 1)

    const sslCertificateIndex = curServerDirectives.value.findIndex(v => v.directive === 'ssl_certificate')
    ensureDirective('ssl_certificate_key', sslCertificateKey, sslCertificateIndex + 1)
  }

  return { ensureTLSDirectives }
}
