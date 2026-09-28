import { describe, expect, test } from 'bun:test'
import { nodeScopeGeneration, onNodeScopeReset, resetNodeScope } from '@/lib/node/nodeScope'

describe('node scope reset', () => {
  test('runs every registered reset', () => {
    const calls: string[] = []
    const offA = onNodeScopeReset(() => calls.push('a'))
    const offB = onNodeScopeReset(() => calls.push('b'))

    resetNodeScope()
    expect(calls).toEqual(['a', 'b'])

    offA()
    offB()
  })

  test('stops calling a reset once unregistered', () => {
    let count = 0
    const off = onNodeScopeReset(() => count++)
    off()

    resetNodeScope()
    expect(count).toBe(0)
  })

  test('bumps the generation so in-flight loads can tell they are stale', () => {
    const before = nodeScopeGeneration()
    resetNodeScope()
    expect(nodeScopeGeneration()).toBe(before + 1)
  })
})
