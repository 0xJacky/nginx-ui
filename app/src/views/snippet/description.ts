import { storeToRefs } from 'pinia'
import { useSettingsStore } from '@/pinia'

/**
 * The description of a snippet in the interface language, falling back to
 * English and then to any language it has.
 */
export function useSnippetDescription() {
  const { language } = storeToRefs(useSettingsStore())
  const current = computed(() => language.value || 'en')

  function describe(description?: Record<string, string>) {
    if (!description)
      return ''
    return description[current.value] ?? description.en ?? Object.values(description)[0] ?? ''
  }

  return { current, describe }
}
