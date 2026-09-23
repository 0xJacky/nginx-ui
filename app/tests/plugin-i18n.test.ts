import { describe, expect, test } from 'bun:test'
import { localizedPluginDescription, localizedPluginName, localizedText } from '../src/api/plugin'

describe('localizedText', () => {
  const values = { en: 'Challenge', zh_CN: '验证', ja: 'チャレンジ' }

  test('prefers the exact locale, then the base language, then English', () => {
    expect(localizedText(values, 'zh_CN')).toBe('验证')
    expect(localizedText(values, 'ja_JP')).toBe('チャレンジ')
    expect(localizedText(values, 'fr_FR')).toBe('Challenge')
  })

  test('uses the fallback before any other translation', () => {
    expect(localizedText({ zh_CN: '验证' }, 'fr_FR', 'Challenge')).toBe('Challenge')
    expect(localizedText({ zh_CN: '验证' }, 'fr_FR')).toBe('验证')
    expect(localizedText(undefined, 'fr_FR')).toBe('')
  })
})

describe('localized plugin fields', () => {
  test('read the flat maps of an installed plugin', () => {
    const info = {
      name: 'DNS-01 Challenge',
      description: 'Solve the challenge.',
      name_i18n: { zh_CN: 'DNS-01 验证' },
      description_i18n: { zh_TW: '完成驗證。' },
    }
    expect(localizedPluginName(info, 'zh_CN')).toBe('DNS-01 验证')
    expect(localizedPluginName(info, 'zh_TW')).toBe('DNS-01 Challenge')
    expect(localizedPluginDescription(info, 'zh_TW')).toBe('完成驗證。')
    expect(localizedPluginDescription(info, 'en')).toBe('Solve the challenge.')
  })

  test('read the i18n block of a manifest and skip empty translations', () => {
    const manifest = {
      name: 'DNS-01 Challenge',
      i18n: {
        ja_JP: { name: 'DNS-01 チャレンジ' },
        zh_CN: { name: '', description: '完成验证。' },
      },
    }
    expect(localizedPluginName(manifest, 'ja_JP')).toBe('DNS-01 チャレンジ')
    expect(localizedPluginName(manifest, 'zh_CN')).toBe('DNS-01 Challenge')
    expect(localizedPluginDescription(manifest, 'zh_CN')).toBe('完成验证。')
    expect(localizedPluginDescription(manifest, 'ja_JP')).toBe('完成验证。')
  })
})
