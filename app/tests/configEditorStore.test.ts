import type { NgxConfig } from '@/api/ngx'
import type { Site } from '@/api/site'
import type { Stream } from '@/api/stream'
import { beforeEach, describe, expect, mock, test } from 'bun:test'
import { createPinia, defineStore, setActivePinia, storeToRefs } from 'pinia'
import { computed, nextTick, ref } from 'vue'

Object.assign(globalThis, { computed, defineStore, nextTick, ref, storeToRefs })

interface PendingRequest<T> {
  name: string
  resolve: (value: T) => void
}

const pendingSites: PendingRequest<Site>[] = []
const pendingStreams: PendingRequest<Stream>[] = []

function enqueue<T>(queue: PendingRequest<T>[], name: string) {
  return new Promise<T>(resolve => {
    queue.push({ name: decodeURIComponent(name), resolve })
  })
}

function release<T>(queue: PendingRequest<T>[], name: string, value: T) {
  const index = queue.findIndex(request => request.name === name)
  if (index < 0)
    throw new Error(`no pending request for ${name}`)
  queue.splice(index, 1)[0].resolve(value)
}

// Only the entity APIs are mocked; mock.module is process wide, so anything the
// other test files also import has to stay untouched.
mock.module('../src/api/site', () => ({
  default: { getItem: (name: string) => enqueue(pendingSites, name) },
}))
mock.module('../src/api/stream', () => ({
  default: { getItem: (name: string) => enqueue(pendingStreams, name) },
}))

const ngxConfigModule = await import('../src/components/NgxConfigEditor/store')
mock.module('../src/components/NgxConfigEditor', () => ngxConfigModule)

const { useSiteEditorStore } = await import('../src/views/site/site_edit/components/SiteEditor/store')
const { useStreamEditorStore } = await import('../src/views/stream/store')

function tokenized(name: string): NgxConfig {
  return { name, servers: [{ directives: [{ directive: 'server_name', params: name }], locations: [] }], upstreams: [] }
}

function siteResponse(name: string, advanced = false): Site {
  return { name, config: `# ${name}`, filepath: `/etc/nginx/sites-available/${name}`, advanced, tokenized: tokenized(name) } as Site
}

function streamResponse(name: string, advanced = false): Stream {
  return { name, config: `# ${name}`, filepath: `/etc/nginx/streams-available/${name}`, advanced, tokenized: tokenized(name) } as Stream
}

beforeEach(() => {
  setActivePinia(createPinia())
  pendingSites.length = 0
  pendingStreams.length = 0
})

describe('site editor store', () => {
  test('clears the previous site before the next one arrives', async () => {
    const store = useSiteEditorStore()

    const first = store.init('a.conf')
    await nextTick()
    release(pendingSites, 'a.conf', siteResponse('a.conf', true))
    await first

    store.dnsLinked = true
    store.linkedDNSName = 'a.example.com'
    expect(store.advanceMode).toBe(true)
    expect(store.configText).toBe('# a.conf')

    const second = store.init('b.conf')
    // The switch must take effect synchronously, before the response lands.
    expect(store.advanceMode).toBe(false)
    expect(store.dnsLinked).toBe(false)
    expect(store.linkedDNSName).toBe('')
    expect(store.configText).toBe('')
    expect(store.data.name).toBeUndefined()
    expect(store.filepath).toBe('')

    await nextTick()
    release(pendingSites, 'b.conf', siteResponse('b.conf'))
    await second

    expect(store.data.name).toBe('b.conf')
    expect(store.name).toBe('b.conf')
    expect(store.advanceMode).toBe(false)
    expect(store.loading).toBe(false)
  })

  test('drops a response for the site the operator already left', async () => {
    const store = useSiteEditorStore()

    const first = store.init('a.conf')
    await nextTick()
    const second = store.init('b.conf')
    await nextTick()

    release(pendingSites, 'b.conf', siteResponse('b.conf'))
    await second
    release(pendingSites, 'a.conf', siteResponse('a.conf', true))
    await first

    expect(store.data.name).toBe('b.conf')
    expect(store.name).toBe('b.conf')
    expect(store.configText).toBe('# b.conf')
    expect(store.advanceMode).toBe(false)
    expect(store.loading).toBe(false)
  })
})

describe('stream editor store', () => {
  test('clears the previous stream before the next one arrives', async () => {
    const store = useStreamEditorStore()

    const first = store.init('a.conf')
    await nextTick()
    release(pendingStreams, 'a.conf', streamResponse('a.conf', true))
    await first

    expect(store.advanceMode).toBe(true)

    const second = store.init('b.conf')
    expect(store.advanceMode).toBe(false)
    expect(store.configText).toBe('')
    expect(store.filepath).toBe('')

    await nextTick()
    release(pendingStreams, 'b.conf', streamResponse('b.conf'))
    await second

    expect(store.data.name).toBe('b.conf')
    expect(store.advanceMode).toBe(false)
    expect(store.loading).toBe(false)
  })

  test('drops a response for the stream the operator already left', async () => {
    const store = useStreamEditorStore()

    const first = store.init('a.conf')
    await nextTick()
    const second = store.init('b.conf')
    await nextTick()

    release(pendingStreams, 'b.conf', streamResponse('b.conf'))
    await second
    release(pendingStreams, 'a.conf', streamResponse('a.conf', true))
    await first

    expect(store.data.name).toBe('b.conf')
    expect(store.name).toBe('b.conf')
    expect(store.configText).toBe('# b.conf')
    expect(store.advanceMode).toBe(false)
  })
})
