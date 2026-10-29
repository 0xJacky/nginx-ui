import { describe, expect, mock, test } from 'bun:test'
import { refusingIngress, upstreamIngressName } from '@/views/upstream/ingressOwner'

const post = mock((..._args: unknown[]) => Promise.resolve({}))
const remove = mock((..._args: unknown[]) => Promise.resolve({}))
mock.module('@uozi-admin/request', () => ({
  http: {
    post,
    delete: remove,
    get: mock(),
  },
}))
const { default: upstream } = await import('@/api/upstream')

describe('ingress-owned upstreams', () => {
  test('names the owning ingress only for groups an ingress generates', () => {
    expect(upstreamIngressName({ managed_by: 'ingress:web' })).toBe('web')
    expect(upstreamIngressName({ managed_by: '' })).toBe('')
    expect(upstreamIngressName({})).toBe('')
  })

  test('reads the ingress from a refused change', () => {
    expect(refusingIngress({
      scope: 'upstream',
      code: 40908,
      params: ['ig-web-app', 'web'],
    })).toBe('web')
    expect(refusingIngress({
      scope: 'upstream',
      code: '40908',
      params: ['ig-web-app', 'web'],
    })).toBe('web')
  })

  test('ignores every other error', () => {
    // Same code in another scope: the ingress API's own conflicts.
    expect(refusingIngress({
      scope: 'ingress',
      code: 40908,
      params: ['a', 'b'],
    })).toBe('')
    expect(refusingIngress({
      scope: 'upstream',
      code: 40903,
      params: ['app'],
    })).toBe('')
    expect(refusingIngress({ scope: 'upstream', code: 40908 })).toBe('')
    expect(refusingIngress(new Error('network'))).toBe('')
    expect(refusingIngress(undefined)).toBe('')
  })

  test('sends the force flag only when the operator chose to edit anyway', async () => {
    const data = {
      name: 'ig-web-app',
      method: '' as const,
      consistent: false,
      keepalive: 0,
      zone: true,
      servers: [],
    }

    await upstream.updateManaged('ig-web-app', data)
    expect(post.mock.calls.at(-1)).toEqual(['/upstreams/ig-web-app', data, undefined])
    await upstream.updateManaged('ig-web-app', data, true)
    expect(post.mock.calls.at(-1)).toEqual([
      '/upstreams/ig-web-app',
      data,
      {
        params: { force: 1 },
      },
    ])

    await upstream.deleteManaged('ig-web-app')
    expect(remove.mock.calls.at(-1)).toEqual(['/upstreams/ig-web-app', undefined])
    await upstream.deleteManaged('ig-web-app', true)
    expect(remove.mock.calls.at(-1)).toEqual([
      '/upstreams/ig-web-app',
      {
        params: { force: 1 },
      },
    ])
  })
})
