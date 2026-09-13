import type { NgxDirective } from '@/api/ngx'

function getServerParams(params: string) {
  return params.trim().split(/\s+/).filter(Boolean)
}

export function hasUpstreamServerAddress(directive: NgxDirective) {
  return directive.directive === 'server' && getServerParams(directive.params).length > 0
}

export function isUpstreamServerEnabled(directive: NgxDirective) {
  const [, ...params] = getServerParams(directive.params)
  return directive.directive !== 'server' || !params.includes('down')
}

export function setUpstreamServerEnabled(directive: NgxDirective, isEnabled: boolean) {
  const [address, ...serverParams] = getServerParams(directive.params)
  if (directive.directive !== 'server' || !address)
    return

  const params = serverParams.filter(param => param !== 'down')

  if (!isEnabled)
    params.push('down')

  directive.params = [address, ...params].join(' ')
}
