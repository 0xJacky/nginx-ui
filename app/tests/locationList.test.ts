import type { NgxLocation } from '@/api/ngx'
import { describe, expect, test } from 'bun:test'
import { appendLocation, duplicateLocation, removeLocation } from '@/components/NgxConfigEditor/locationList'

const root: NgxLocation = { path: '/', content: 'proxy_pass http://127.0.0.1:5000;', comments: '' }
const authz: NgxLocation = {
  path: '= /internal/authelia/authz',
  content: 'internal;\nproxy_pass http://127.0.0.1:9091/api/authz/auth-request;',
  comments: '',
}
const challenge: NgxLocation = { path: '~ /.well-known/acme-challenge', content: 'proxy_pass http://127.0.0.1:9180;', comments: '' }

describe('appendLocation', () => {
  test.each([
    {
      name: 'server without locations (null)',
      locations: null,
      want: [authz],
    },
    {
      name: 'server without locations (undefined)',
      locations: undefined,
      want: [authz],
    },
    {
      name: 'empty list',
      locations: [],
      want: [authz],
    },
    {
      name: 'after existing locations',
      locations: [root, challenge],
      want: [root, challenge, authz],
    },
  ])('$name', ({ locations, want }) => {
    const got = appendLocation(locations, authz)

    expect(got).toEqual(want)
    // A new list, so assigning it to the v-model always updates the server.
    expect(got).not.toBe(locations as unknown as NgxLocation[])
  })

  test('leaves the given list unchanged', () => {
    const locations = [root]
    appendLocation(locations, authz)
    expect(locations).toEqual([root])
  })
})

describe('removeLocation', () => {
  test.each([
    {
      name: 'null list',
      locations: null,
      index: 0,
      want: [],
    },
    {
      name: 'first location',
      locations: [root, authz, challenge],
      index: 0,
      want: [authz, challenge],
    },
    {
      name: 'middle location',
      locations: [root, authz, challenge],
      index: 1,
      want: [root, challenge],
    },
    {
      name: 'out of range index',
      locations: [root],
      index: 3,
      want: [root],
    },
  ])('$name', ({ locations, index, want }) => {
    expect(removeLocation(locations, index)).toEqual(want)
  })
})

describe('duplicateLocation', () => {
  test.each([
    {
      name: 'null list',
      locations: null,
      index: 0,
      want: [],
    },
    {
      name: 'copy goes before the original',
      locations: [root, authz],
      index: 1,
      want: [root, authz, authz],
    },
    {
      name: 'out of range index',
      locations: [root],
      index: 2,
      want: [root],
    },
  ])('$name', ({ locations, index, want }) => {
    expect(duplicateLocation(locations, index)).toEqual(want)
  })

  test('the copy is independent of the original', () => {
    const got = duplicateLocation([authz], 0)

    got[0].content = 'changed'
    expect(got[1].content).toBe(authz.content)
    expect(got[0]).not.toBe(got[1])
  })
})
