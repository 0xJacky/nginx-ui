import type { NgxServer } from '@/api/ngx'
import { describe, expect, test } from 'bun:test'
import {
  defaultEditorPanelKeys,
  EDITOR_PANEL_KEY,
  isModeLocked,
  sslStepFinishLabel,
  sslStepTLSState,
} from '@/views/site/site_add/wizardSteps'

Object.assign(globalThis, { $gettext: (message: string) => message })

function httpServer(): NgxServer {
  return {
    directives: [
      { directive: 'listen', params: '80' },
      { directive: 'server_name', params: 'app.example.com' },
    ],
    locations: [{ path: '/', content: 'proxy_pass http://127.0.0.1:3000;', comments: '' }],
  }
}

function tlsServer(certificate = ''): NgxServer {
  return {
    directives: [
      { directive: 'listen', params: '443 ssl' },
      { directive: 'server_name', params: 'app.example.com' },
      { directive: 'ssl_certificate', params: certificate ? `${certificate}.cer` : '' },
      { directive: 'ssl_certificate_key', params: certificate ? `${certificate}.key` : '' },
    ],
    locations: [],
  }
}

describe('mode selector', () => {
  test('the mode can only change on the first step', () => {
    expect(isModeLocked(0)).toBe(false)
    expect(isModeLocked(1)).toBe(true)
    expect(isModeLocked(2)).toBe(true)
    expect(isModeLocked(3)).toBe(true)
  })
})

describe('configuration editor panel', () => {
  test('collapsed in Quick Setup, expanded in Advanced', () => {
    expect(defaultEditorPanelKeys('quick')).toEqual([])
    expect(defaultEditorPanelKeys('advanced')).toEqual([EDITOR_PANEL_KEY])
  })

  test('each call returns a fresh list the collapse may mutate', () => {
    expect(defaultEditorPanelKeys('advanced')).not.toBe(defaultEditorPanelKeys('advanced'))
  })
})

describe('sslStepTLSState', () => {
  test('plain HTTP has no TLS server', () => {
    expect(sslStepTLSState({ servers: [httpServer()] })).toBe('none')
    expect(sslStepTLSState({ servers: [] })).toBe('none')
    expect(sslStepTLSState(undefined)).toBe('none')
  })

  test('a TLS server without certificate is pending', () => {
    expect(sslStepTLSState({ servers: [httpServer(), tlsServer()] })).toBe('pending')
  })

  test('one pending TLS server outweighs a configured one', () => {
    expect(sslStepTLSState({ servers: [httpServer(), tlsServer('/etc/ssl/app'), tlsServer()] })).toBe('pending')
  })

  test('TLS servers that all have a certificate are configured', () => {
    expect(sslStepTLSState({ servers: [httpServer(), tlsServer('/etc/ssl/app')] })).toBe('configured')
  })
})

describe('sslStepFinishLabel', () => {
  test('skipping only finishes the wizard', () => {
    expect(sslStepFinishLabel('skip', 'idle')).toBe('Finish')
    expect(sslStepFinishLabel(undefined, undefined)).toBe('Finish')
  })

  test('names the HTTPS work the button starts', () => {
    expect(sslStepFinishLabel('http01', 'idle')).toBe('Issue certificate and finish')
    expect(sslStepFinishLabel('dns01', 'running')).toBe('Issue certificate and finish')
    expect(sslStepFinishLabel('existing', 'idle')).toBe('Enable HTTPS and finish')
  })

  test('a failed run is retried', () => {
    expect(sslStepFinishLabel('http01', 'error')).toBe('Retry')
    expect(sslStepFinishLabel('existing', 'error')).toBe('Retry')
  })
})
