<script setup lang="ts">
import { DeleteOutlined, TranslationOutlined } from '@antdv-next/icons'
import gettext from '@/gettext'
import { localize, useSnippetDescription } from '../description'

const props = defineProps<{
  placeholder?: string
  maxlength?: number
}>()

// Text per language. The input edits the interface language; the others are
// edited as translations.
const text = defineModel<Record<string, string>>({ required: true })

const { current: language } = useSnippetDescription()
const languageNames = gettext.available as Record<string, string>

const isOpen = ref(false)
// Languages added in this session that have no text yet.
const added = ref<string[]>([])

function setText(lang: string, value: string) {
  const next = { ...text.value }
  if (value.trim())
    next[lang] = value
  else
    delete next[lang]
  text.value = next
}

const currentText = computed({
  get: () => text.value[language.value] ?? '',
  set: value => setText(language.value, value),
})

// Without text in the interface language, the text another language has is
// shown as the placeholder.
const placeholder = computed(() => localize(text.value, language.value) || props.placeholder)

// Chinese is told apart by script, other languages by the language alone.
function toTag(lang: string) {
  if (lang === 'zh_CN')
    return 'zh-Hans'
  if (lang === 'zh_TW')
    return 'zh-Hant'
  return lang.split('_')[0]
}

const displayNames = computed(() => {
  try {
    return new Intl.DisplayNames([toTag(language.value)], { type: 'language' })
  }
  catch {
    return undefined
  }
})

function nameOf(lang: string) {
  try {
    return displayNames.value?.of(toTag(lang)) ?? languageNames[lang] ?? lang
  }
  catch {
    return languageNames[lang] ?? lang
  }
}

const translations = computed(() => [...new Set([...Object.keys(text.value), ...added.value])]
  .filter(lang => lang !== language.value)
  .sort((a, b) => nameOf(a).localeCompare(nameOf(b))))

const translationCount = computed(() => Object.keys(text.value).filter(lang => lang !== language.value).length)

const addOptions = computed(() => Object.keys(languageNames)
  .filter(lang => lang !== language.value && !translations.value.includes(lang))
  .map(lang => ({ label: nameOf(lang), value: lang })))

function addLanguage(lang: string) {
  added.value.push(lang)
}

function removeLanguage(lang: string) {
  added.value = added.value.filter(l => l !== lang)
  setText(lang, '')
}
</script>

<template>
  <AInput
    v-model:value="currentText"
    :maxlength="maxlength"
    :placeholder="placeholder"
  >
    <template #suffix>
      <APopover
        v-model:open="isOpen"
        trigger="click"
        placement="bottomRight"
        :title="$gettext('Translations')"
        :styles="{ container: { width: '360px', maxWidth: 'calc(100vw - 32px)' } }"
      >
        <AButton
          type="text"
          size="small"
          class="translation-trigger"
          :aria-label="$gettext('Translations')"
        >
          <TranslationOutlined />
          <span v-if="translationCount > 0">{{ translationCount }}</span>
        </AButton>
        <template #content>
          <div class="flex flex-col gap-2">
            <div class="hint text-sm">
              {{ $gettext('The field is in %{language}. Users of other languages see their translation, or English when there is none.', { language: nameOf(language) }) }}
            </div>
            <div
              v-for="lang in translations"
              :key="lang"
              class="flex flex-col gap-1"
            >
              <span class="hint text-[13px]">{{ nameOf(lang) }}</span>
              <div class="flex items-center gap-1">
                <AInput
                  :value="text[lang] ?? ''"
                  :maxlength="maxlength"
                  :aria-label="nameOf(lang)"
                  @update:value="setText(lang, $event)"
                />
                <AButton
                  type="text"
                  :aria-label="$gettext('Remove translation')"
                  @click="removeLanguage(lang)"
                >
                  <template #icon>
                    <DeleteOutlined />
                  </template>
                </AButton>
              </div>
            </div>
            <ASelect
              v-if="addOptions.length > 0"
              :value="undefined"
              :options="addOptions"
              :placeholder="$gettext('Add a language')"
              show-search
              option-filter-prop="label"
              @change="addLanguage($event as string)"
            />
          </div>
        </template>
      </APopover>
    </template>
  </AInput>
</template>

<style scoped lang="less">
.translation-trigger {
  margin-inline-end: -4px;
  color: var(--ant-color-text-tertiary);
}

.hint {
  color: var(--ant-color-text-secondary);
}
</style>
