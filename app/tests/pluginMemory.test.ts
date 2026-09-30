import { describe, expect, test } from 'bun:test'

function fakeGettext(text: string, params: Record<string, string> = {}) {
  return text.replace(/%\{(\w+)\}/g, (_, key) => params[key] ?? '')
}
Object.assign(globalThis, { $gettext: fakeGettext, $pgettext: (_context: string, text: string) => text })

const { formatMemory, isBelowRecommended, memoryWarning, recommendedMemory } = await import('../src/views/system/plugins/memory')

describe('formatMemory', () => {
  test('uses MB below one GiB and GB above', () => {
    expect(formatMemory(512)).toBe('512 MB')
    expect(formatMemory(1024)).toBe('1 GB')
    expect(formatMemory(1536)).toBe('1.5 GB')
  })
})

describe('recommendedMemory', () => {
  test('reads the hint and treats missing or bad values as none', () => {
    expect(recommendedMemory({ server: { resources: { recommended_memory_mb: 512 } } })).toBe(512)
    expect(recommendedMemory({ server: { resources: { recommended_memory_mb: -1 } } })).toBe(0)
    expect(recommendedMemory({ server: {} })).toBe(0)
    expect(recommendedMemory(undefined)).toBe(0)
  })
})

describe('isBelowRecommended', () => {
  test('warns only when both values are known and memory is short', () => {
    expect(isBelowRecommended(512, 256)).toBe(true)
    expect(isBelowRecommended(512, 512)).toBe(false)
    expect(isBelowRecommended(512, 0)).toBe(false)
    expect(isBelowRecommended(0, 256)).toBe(false)
    expect(isBelowRecommended(undefined, undefined)).toBe(false)
  })
})

describe('memoryWarning', () => {
  test('names both sizes without technical terms', () => {
    const text = memoryWarning(512, 256)
    expect(text).toContain('256 MB')
    expect(text).toContain('512 MB')
  })
})
