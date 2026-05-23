import { storeToRefs } from 'pinia'
import { useSiteEditorStore } from '../components/SiteEditor/store'

function getListenPort(params: string) {
  const firstToken = params.trim().split(/\s+/)[0]?.replace(/;$/, '')
  if (!firstToken)
    return ''

  const ipv6Port = firstToken.match(/^\[[^\]]+\]:(\d+)$/)
  if (ipv6Port)
    return ipv6Port[1]

  const port = firstToken.match(/(?:^|:)(\d+)$/)
  return port?.[1] ?? ''
}

export function hasTLSListen(params: string) {
  const tokens = params.trim().split(/\s+/).map(token => token.replace(/;$/, ''))
  return getListenPort(params) === '443' && tokens.includes('ssl')
}

export function isIPv6Listen(params: string) {
  return params.trim().startsWith('[')
}

// useTLSDirectives provides helpers that write SSL directives into the
// currently edited server block.
export function useTLSDirectives() {
  const editorStore = useSiteEditorStore()
  const { curServerDirectives, curDirectivesMap } = storeToRefs(editorStore)

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

    const hasIPv4TLSListen = curServerDirectives.value.some(v => v.directive === 'listen' && hasTLSListen(v.params) && !isIPv6Listen(v.params))
    const hasIPv6TLSListen = curServerDirectives.value.some(v => v.directive === 'listen' && hasTLSListen(v.params) && isIPv6Listen(v.params))

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
