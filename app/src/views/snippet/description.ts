import { storeToRefs } from 'pinia'
import { useSettingsStore } from '@/pinia'

type Localized = Record<string, string>

/** The text of a localized field in a language, falling back to English and then to any language it has. */
export function localize(text: Localized | undefined, language: string) {
  if (!text)
    return ''
  return text[language] ?? text.en ?? Object.values(text)[0] ?? ''
}

/**
 * The description of a snippet in the interface language, falling back to
 * English and then to any language it has.
 */
export function useSnippetDescription() {
  const { language } = storeToRefs(useSettingsStore())
  const current = computed(() => language.value || 'en')

  function describe(description?: Localized) {
    return localize(description, current.value)
  }

  return { current, describe }
}
