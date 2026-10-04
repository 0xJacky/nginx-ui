import dayjs from 'dayjs'

// App language -> dayjs locale.
const localeMap: Record<string, string> = {
  fr: 'fr',
  ja: 'ja',
  ko: 'ko',
  de: 'de',
  zh_CN: 'zh-cn',
  zh_TW: 'zh-tw',
  pt: 'pt',
  es: 'es',
  it: 'it',
  it_IT: 'it',
  ar: 'ar',
  ru: 'ru',
  tr: 'tr',
  vi: 'vi',
  uk_UA: 'uk',
}

// Predefined locale importers for dynamic loading
// This approach works with Vite's static analysis requirements
const localeImporters: Record<string, () => Promise<unknown>> = {
  'fr': () => import('dayjs/locale/fr'),
  'ja': () => import('dayjs/locale/ja'),
  'ko': () => import('dayjs/locale/ko'),
  'de': () => import('dayjs/locale/de'),
  'zh-cn': () => import('dayjs/locale/zh-cn'),
  'zh-tw': () => import('dayjs/locale/zh-tw'),
  'pt': () => import('dayjs/locale/pt'),
  'es': () => import('dayjs/locale/es'),
  'it': () => import('dayjs/locale/it'),
  'ar': () => import('dayjs/locale/ar'),
  'ru': () => import('dayjs/locale/ru'),
  'tr': () => import('dayjs/locale/tr'),
  'vi': () => import('dayjs/locale/vi'),
  'uk': () => import('dayjs/locale/uk'),
}

/** Switches dayjs to the locale of an app language, falling back to English. */
export async function loadDayjsLocale(language: string) {
  const dayjsLocale = localeMap[language]

  if (!dayjsLocale) {
    dayjs.locale('en')
    return
  }

  try {
    const importer = localeImporters[dayjsLocale]
    if (importer) {
      await importer()
      dayjs.locale(dayjsLocale)
    }
    else {
      dayjs.locale('en')
    }
  }
  catch (error) {
    console.warn(`Failed to load dayjs locale: ${dayjsLocale}`, error)
    dayjs.locale('en')
  }
}
