import i18n from '../../../i18n.json'

// Mapping from BCP-47 language tags to supported project language codes
const LOCALE_MAP: Record<string, string> = {
  // Chinese variants
  'zh': 'zh_CN',
  'zh-hans': 'zh_CN',
  'zh-hant': 'zh_TW',
  'zh-cn': 'zh_CN',
  'zh-sg': 'zh_CN',
  'zh-my': 'zh_CN',
  'zh-tw': 'zh_TW',
  'zh-hk': 'zh_TW',
  'zh-mo': 'zh_TW',
  // Other supported languages
  'ar': 'ar',
  'de': 'de_DE',
  'en': 'en',
  'es': 'es',
  'fr': 'fr_FR',
  'ja': 'ja_JP',
  'ko': 'ko_KR',
  'pt': 'pt_PT',
  'ru': 'ru_RU',
  'tr': 'tr_TR',
  'uk': 'uk_UA',
  'vi': 'vi_VN',
}

const SUPPORTED_LANGUAGES = Object.keys(i18n)

interface ParsedLocale {
  base: string
  full: string
}

/**
 * Parse a locale string into its base language and normalized full tag.
 * Falls back to simple string splitting if Intl.Locale is unavailable.
 */
function parseLocale(locale: string): ParsedLocale {
  const normalized = locale.toLowerCase().replace(/_/g, '-').trim()

  try {
    const intlLocale = new Intl.Locale(normalized)
    return {
      base: intlLocale.language.toLowerCase(),
      full: normalized,
    }
  }
  catch {
    const parts = normalized.split('-')
    return {
      base: parts[0] || normalized,
      full: normalized,
    }
  }
}

/**
 * Detect the best matching supported language from the browser preferences.
 * Returns an empty string if no supported language matches.
 */
export function getBrowserLanguage(): string {
  const candidates = [
    navigator.language,
    ...(navigator.languages || []),
  ].filter(Boolean) as string[]

  for (const candidate of candidates) {
    const { base, full } = parseLocale(candidate)

    // Prefer full locale match (e.g. zh-CN, zh-TW)
    if (LOCALE_MAP[full]) {
      return LOCALE_MAP[full]
    }

    // Fall back to base language match (e.g. zh -> zh_CN)
    if (LOCALE_MAP[base]) {
      return LOCALE_MAP[base]
    }

    // Direct match against supported project codes
    const directMatch = SUPPORTED_LANGUAGES.find(lang =>
      lang.toLowerCase() === full || lang.toLowerCase().startsWith(`${base}_`),
    )
    if (directMatch) {
      return directMatch
    }
  }

  return ''
}
