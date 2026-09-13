import type { NgxDirective } from '@/api/ngx'

function getServerParams(params: string) {
  return params.trim().split(/\s+/).filter(Boolean)
}

export function isUpstreamServerEnabled(directive: NgxDirective) {
  return directive.directive !== 'server'
    || !getServerParams(directive.params).includes('down')
}

export function setUpstreamServerEnabled(directive: NgxDirective, isEnabled: boolean) {
  const params = getServerParams(directive.params).filter(param => param !== 'down')

  if (!isEnabled)
    params.push('down')

  directive.params = params.join(' ')
}
