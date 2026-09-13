import type { NgxDirective } from '@/api/ngx'
import { describe, expect, test } from 'bun:test'
import {
  isUpstreamServerEnabled,
  setUpstreamServerEnabled,
} from '@/components/NgxConfigEditor/upstreamServer'

function server(params: string): NgxDirective {
  return { directive: 'server', params }
}

describe('upstream server toggle', () => {
  test('detects only the standalone down parameter', () => {
    expect(isUpstreamServerEnabled(server('127.0.0.1:8080 weight=2'))).toBe(true)
    expect(isUpstreamServerEnabled(server('download.example:8080'))).toBe(true)
    expect(isUpstreamServerEnabled(server('127.0.0.1:8080 down'))).toBe(false)
  })

  test('disables a server without duplicating down', () => {
    const directive = server('127.0.0.1:8080 weight=2')

    setUpstreamServerEnabled(directive, false)
    setUpstreamServerEnabled(directive, false)

    expect(directive.params).toBe('127.0.0.1:8080 weight=2 down')
  })

  test('enables a server while preserving its other parameters', () => {
    const directive = server('127.0.0.1:8080  down max_fails=2 down')

    setUpstreamServerEnabled(directive, true)

    expect(directive.params).toBe('127.0.0.1:8080 max_fails=2')
  })
})
