import type { SyncTask } from '../src/api/cluster_sync'
import { afterEach, beforeEach, describe, expect, mock, test } from 'bun:test'

Object.assign(globalThis, {
  $gettext: (message: string) => message,
})

type Handler = (data: SyncTask) => void
const handlers = new Map<string, Handler>()
let nextSubscription = 0
const unsubscribe = mock((id: string) => {
  handlers.delete(id)
})

const getSyncTask = mock((_id: string): Promise<SyncTask> => new Promise(() => undefined))

mock.module('../src/api/cluster_sync', () => ({
  SYNC_TASK_EVENT: 'cluster_sync_task',
  getSyncTask,
}))
mock.module('../src/pinia', () => ({
  useWebSocketEventBusStore: () => ({
    subscribe: (_event: string, handler: Handler) => {
      const id = String(nextSubscription++)
      handlers.set(id, handler)
      return id
    },
    unsubscribe,
  }),
}))

const { waitForSyncTask } = await import('../src/composables/useClusterSync')

const summary = {
  total: 2,
  succeeded: 2,
  failed: 0,
  results: [],
}

function task(overrides: Partial<SyncTask> = {}): SyncTask {
  return {
    id: 'task-1',
    label: 'production',
    status: 'running',
    done: 0,
    total: 2,
    started_at: '2026-11-17T00:00:00Z',
    ...overrides,
  }
}

function emit(data: SyncTask) {
  for (const handler of [
    ...handlers.values(),
  ])
    handler(data)
}

describe('waitForSyncTask', () => {
  beforeEach(() => {
    handlers.clear()
    unsubscribe.mockClear()
    getSyncTask.mockClear()
  })

  afterEach(() => {
    handlers.clear()
  })

  test('reports progress and resolves with the summary from the finish event', async () => {
    const progress: number[] = []
    const result = waitForSyncTask(task(), current => progress.push(current.done))

    emit(task({ done: 1 }))
    emit(task({
      id: 'another-task',
      status: 'finished',
      summary: { ...summary, failed: 9 },
    }))
    emit(task({ status: 'finished', done: 2, summary }))

    expect(await result).toEqual(summary)
    expect(progress).toEqual([0, 1])
    expect(unsubscribe).toHaveBeenCalledTimes(1)
    expect(handlers.size).toBe(0)
  })

  test('resolves at once for a task that has already finished', async () => {
    expect(await waitForSyncTask(task({ status: 'finished', summary }))).toEqual(summary)
    expect(handlers.size).toBe(0)
  })
})
