import countries from 'i18n-iso-countries'
import ar from 'i18n-iso-countries/langs/ar.json'
import de from 'i18n-iso-countries/langs/de.json'
import en from 'i18n-iso-countries/langs/en.json'
import es from 'i18n-iso-countries/langs/es.json'
import fr from 'i18n-iso-countries/langs/fr.json'
import ja from 'i18n-iso-countries/langs/ja.json'
import ko from 'i18n-iso-countries/langs/ko.json'
import pt from 'i18n-iso-countries/langs/pt.json'
import ru from 'i18n-iso-countries/langs/ru.json'
import tr from 'i18n-iso-countries/langs/tr.json'
import uk from 'i18n-iso-countries/langs/uk.json'
import vi from 'i18n-iso-countries/langs/vi.json'
import zhCN from 'i18n-iso-countries/langs/zh.json'
import { useSettingsStore } from '@/pinia'

// Register all supported languages
countries.registerLocale(en)
countries.registerLocale(zhCN)
countries.registerLocale(fr)
countries.registerLocale(es)
countries.registerLocale(de)
countries.registerLocale(ru)
countries.registerLocale(vi)
countries.registerLocale(ko)
countries.registerLocale(tr)
countries.registerLocale(ar)
countries.registerLocale(uk)
countries.registerLocale(ja)
countries.registerLocale(pt)

export interface GeoData {
  code: string
  name?: string
  region?: string
  province?: string
  city?: string
  isp?: string
  value: number
  percent: number
}

export function useGeoTranslation() {
  const settings = useSettingsStore()

  // Language mapping from project language codes to i18n-iso-countries codes
  const getLanguageCode = (settingsLang: string): string => {
    const langMap: Record<string, string> = {
      en: 'en',
      zh_CN: 'zh',
      zh_TW: 'zh', // Note: i18n-iso-countries uses 'zh' for both
      fr_FR: 'fr',
      es: 'es',
      de_DE: 'de',
      ru_RU: 'ru',
      vi_VN: 'vi',
      ko_KR: 'ko',
      tr_TR: 'tr',
      ar: 'ar',
      uk_UA: 'uk',
      ja_JP: 'ja',
      pt_PT: 'pt',
    }
    return langMap[settingsLang] || 'en'
  }

  // Get current locale from settings store, fallback to browser language
  const getCurrentLocale = (): string => {
    const settingsLang = settings.language
    if (settingsLang) {
      return getLanguageCode(settingsLang)
    }

    // Fallback to browser language if settings not available
    const browserLocale = navigator.language.toLowerCase()
    if (browserLocale.startsWith('zh-cn') || browserLocale.startsWith('zh-hans')) {
      return 'zh'
    }
    if (browserLocale.startsWith('zh-tw') || browserLocale.startsWith('zh-hant')) {
      return 'zh'
    }
    // Map other browser languages to supported codes
    const browserLangCode = browserLocale.split('-')[0]
    const supportedLangs = ['fr', 'es', 'de', 'ru', 'vi', 'ko', 'tr', 'ar', 'uk', 'ja', 'pt']
    if (supportedLangs.includes(browserLangCode)) {
      return browserLangCode
    }
    return 'en'
  }

  const locale = computed(() => getCurrentLocale())
  const isChineseLocale = computed(() => {
    const settingsLang = settings.language
    return settingsLang === 'zh_CN' || settingsLang === 'zh_TW'
  })

  // Translate country code to localized name
  const translateCountry = (countryCode: string): string => {
    if (!countryCode)
      return ''

    // Handle special cases
    if (countryCode === 'UNKNOWN') {
      return isChineseLocale.value ? '未知' : 'Unknown'
    }

    const currentLocale = locale.value
    return countries.getName(countryCode, currentLocale) || countryCode
  }

  // Format geographic display based on locale
  const formatGeoDisplay = (data: GeoData): string => {
    if (isChineseLocale.value) {
      // For Chinese locales, show detailed geographic info
      const parts: string[] = []

      if (data.province)
        parts.push(data.province)
      if (data.city)
        parts.push(data.city)

      return parts.length > 0 ? parts.join(' ') : translateCountry(data.code)
    }
    else {
      // For other locales, only show country name
      return translateCountry(data.code)
    }
  }

  // Format tooltip content based on locale
  const formatTooltip = (data: GeoData): string => {
    const countryName = translateCountry(data.code)

    if (isChineseLocale.value) {
      // Show detailed info for Chinese locales
      let content = `<div style="font-size: 14px;"><strong>${countryName}</strong><br/>`

      if (data.province)
        content += `省份: ${data.province}<br/>`
      if (data.city)
        content += `城市: ${data.city}<br/>`
      if (data.isp)
        content += `ISP: ${data.isp}<br/>`

      content += `访问量: ${data.value || 0}<br/>`
      content += `占比: ${(data.percent || 0).toFixed(2)}%</div>`

      return content
    }
    else {
      // Show only country and stats for other locales
      return `
        <div style="font-size: 14px;">
          <strong>${countryName}</strong><br/>
          Visits: ${data.value || 0}<br/>
          Percentage: ${(data.percent || 0).toFixed(2)}%
        </div>
      `
    }
  }

  // Get labels using gettext
  const getLabels = () => {
    return {
      high: $gettext('High'),
      low: $gettext('Low'),
      visits: $gettext('Visits'),
      percentage: $gettext('Percentage'),
      noData: $gettext('No data'),
    }
  }

  return {
    locale,
    isChineseLocale,
    translateCountry,
    formatGeoDisplay,
    formatTooltip,
    getLabels,
  }
}
