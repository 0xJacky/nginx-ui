import type { NgxLocation, NgxServer } from '@/api/ngx'
import { describe, expect, it } from 'vitest'
import { ensureHTTPChallengeLocation } from '@/views/site/site_edit/composables/useHTTPChallenge'

const challengeLocation: NgxLocation = {
  path: '~ /.well-known/acme-challenge',
  content: 'proxy_pass http://127.0.0.1:9180;',
  comments: '',
}

describe('ensureHTTPChallengeLocation', () => {
  it('moves a server-level return into the catch-all location', () => {
    const server: NgxServer = {
      directives: [
        { directive: 'return', params: '301 https://$host$request_uri' },
      ],
      locations: [
        { path: '/', content: 'proxy_pass http://127.0.0.1:9000;', comments: '' },
      ],
    }

    ensureHTTPChallengeLocation(server, [challengeLocation])

    expect(server.directives).toEqual([])
    expect(server.locations).toEqual([
      {
        path: '/',
        content: 'return 301 https://$host$request_uri;\nproxy_pass http://127.0.0.1:9000;',
        comments: '',
      },
      challengeLocation,
    ])
  })

  it('replaces an existing challenge location instead of duplicating it', () => {
    const server: NgxServer = {
      locations: [challengeLocation],
    }

    ensureHTTPChallengeLocation(server, [challengeLocation])

    expect(server.locations).toHaveLength(1)
    expect(server.locations?.[0].path).toBe(challengeLocation.path)
  })
})
