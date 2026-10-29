import { describe, expect, test } from 'bun:test'
import { LLM_PROVIDERS, suggestThinkingPreset } from '../src/constants/llm'

describe('MiniMax presets', () => {
  test('offers only the current model pair with Flash first', () => {
    const models = LLM_PROVIDERS.find(provider => provider.value === 'minimax')?.models
    expect(models?.map(model => model.value)).toEqual(['MiniMax-M3.1-Flash', 'MiniMax-M3'])
    expect(models?.[0].thinkingModes).toEqual(['adaptive'])
    expect(models?.[0].pricing).toEqual({ input: 0.15, output: 0.6, cacheRead: 0.015 })
    expect(models?.[1].pricingTiers?.map(tier => tier.pricing)).toEqual([
      { input: 0.3, output: 1.2, cacheRead: 0.06 },
      { input: 0.6, output: 2.4, cacheRead: 0.12 },
      { input: 0.45, output: 1.8, cacheRead: 0.09 },
      { input: 0.9, output: 3.6, cacheRead: 0.18 },
    ])
  })

  test('never offers disabled thinking or none effort for Flash', () => {
    const preset = suggestThinkingPreset('MiniMax-M3.1-Flash', 'https://api.minimax.io/v1')
    expect(preset.value).toBe('minimax_flash')
    expect(preset.params?.off).toBeUndefined()
    for (const level of ['low', 'medium', 'high'] as const) {
      expect(preset.params?.[level]).toEqual({
        thinking: { type: 'adaptive' },
        reasoning_effort: level,
      })
    }
    expect(suggestThinkingPreset('MiniMax-M3.1-Flash', 'https://api.minimaxi.com/v1').value).toBe('minimax_flash')
  })

  test('keeps M3 thinking switch separate from Flash effort controls', () => {
    const preset = suggestThinkingPreset('MiniMax-M3')
    expect(preset.value).toBe('minimax_m3')
    expect(preset.params?.off).toEqual({
      thinking: { type: 'disabled' },
    })
    expect(preset.params?.high).toEqual({
      thinking: { type: 'adaptive' },
    })
    expect(preset.params?.low).toBeUndefined()
  })

  test('preserves endpoint precedence and does not match retired or malformed IDs', () => {
    expect(suggestThinkingPreset('MiniMax-M3.1-Flash', 'http://localhost:11434/v1').value).toBe('openai')
    for (const name of ['MiniMax-M2.7', 'MiniMax-M3x1-Flash', 'MiniMax-M3.1-Flash-extra']) {
      expect(suggestThinkingPreset(name).value).toBe('none')
    }
    expect(suggestThinkingPreset('gpt-5').value).toBe('openai')
    expect(suggestThinkingPreset('deepseek-chat').value).toBe('thinking_type')
  })
})
