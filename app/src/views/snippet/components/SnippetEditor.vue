<script setup lang="ts">
import type { Snippet } from '@/api/snippet'
import snippet from '@/api/snippet'
import CodeEditor from '@/components/CodeEditor'
import { useSnippetDescription } from '../description'
import SnippetUsage from './SnippetUsage.vue'

const props = defineProps<{
  // File of the snippet to edit; undefined to create a new one.
  file?: string
}>()

const emit = defineEmits<{
  saved: [snippet: Snippet]
}>()

const open = defineModel<boolean>('open', { default: false })

const { message } = App.useApp()
const { current: language } = useSnippetDescription()

interface FormState {
  baseName: string
  name: string
  description: string
  content: string
}

function createForm(): FormState {
  return { baseName: '', name: '', description: '', content: '' }
}

const form = reactive<FormState>(createForm())
const loaded = ref<Snippet>()
const isBaseNameTouched = ref(false)
const isLoading = ref(false)
const isSaving = ref(false)

const isEditing = computed(() => props.file !== undefined)
const fileName = computed(() => isEditing.value ? props.file! : `${form.baseName.trim()}.conf`)
const variableNames = computed(() => Object.keys(loaded.value?.variables ?? {}))
// A snippet with variables is filled in by the site editor, never included.
const include = computed(() => (form.baseName.trim() || isEditing.value) && variableNames.value.length === 0
  ? `include snippets/${fileName.value};`
  : '')

function toBaseName(name: string) {
  return name.toLowerCase().replace(/[^a-z0-9._-]+/g, '-').replace(/^[^a-z0-9]+|-+$/g, '').slice(0, 64)
}

watch(() => form.name, name => {
  if (!isEditing.value && !isBaseNameTouched.value)
    form.baseName = toBaseName(name)
})

async function load() {
  Object.assign(form, createForm())
  loaded.value = undefined
  isBaseNameTouched.value = false

  if (props.file === undefined)
    return

  isLoading.value = true
  try {
    const detail = await snippet.get(props.file)
    loaded.value = detail
    Object.assign(form, {
      baseName: detail.file.replace(/\.conf$/, ''),
      name: detail.name,
      description: detail.description[language.value] ?? detail.description.en ?? '',
      content: detail.content ?? '',
    })
  }
  catch {
    open.value = false
  }
  finally {
    isLoading.value = false
  }
}

watch(open, value => {
  if (value)
    load()
}, { immediate: true })

async function save() {
  isSaving.value = true
  try {
    // The description is kept per language: this edit replaces the one of
    // the interface language and keeps the others.
    const description = { ...loaded.value?.description }
    if (form.description.trim())
      description[language.value] = form.description.trim()
    else
      delete description[language.value]

    const payload = { name: form.name.trim(), description, content: form.content }
    const saved = isEditing.value
      ? await snippet.update(props.file!, payload)
      : await snippet.create({ ...payload, file: fileName.value })
    message.success($gettext('Snippet %{name} saved', { name: saved.name }))
    emit('saved', saved)
    open.value = false
  }
  catch {
    // The request layer already shows the error, including the nginx -t output.
  }
  finally {
    isSaving.value = false
  }
}
</script>

<template>
  <ADrawer
    v-model:open="open"
    :title="isEditing ? $gettext('Edit Snippet %{name}', { name: form.name || fileName }) : $gettext('Create Snippet')"
    :size="760"
    destroy-on-hidden
  >
    <ASpin :spinning="isLoading">
      <AForm layout="vertical">
        <AFlex gap="middle" wrap>
          <AFormItem :label="$gettext('Name')" class="min-w-60 flex-1">
            <AInput
              v-model:value="form.name"
              :maxlength="100"
              :placeholder="$gettext('Static file cache')"
            />
          </AFormItem>
          <AFormItem
            :label="$gettext('File Name')"
            required
            class="min-w-60 flex-1"
            :extra="isEditing
              ? $gettext('Fixed after creation, so the sites that include the snippet keep working.')
              : $gettext('Letters, digits, dots, dashes and underscores.')"
          >
            <AInput
              v-model:value="form.baseName"
              class="font-mono"
              :disabled="isEditing"
              placeholder="static-cache"
              @input="isBaseNameTouched = true"
            >
              <template #suffix>
                <span class="font-mono text-gray-500 dark:text-gray-400">.conf</span>
              </template>
            </AInput>
          </AFormItem>
        </AFlex>

        <AFormItem :label="$gettext('Description')">
          <AInput
            v-model:value="form.description"
            :maxlength="200"
          />
        </AFormItem>

        <AAlert
          v-if="variableNames.length > 0"
          class="mb-4"
          type="info"
          show-icon
          :title="$gettext('This snippet has variables: %{names}. Insert it from the config template panel of the site editor, which fills them in; Nginx cannot include it directly.', { names: variableNames.join(', ') })"
        />

        <AFormItem :label="$gettext('Content')">
          <CodeEditor
            v-model:content="form.content"
            default-height="320px"
          />
          <template v-if="include" #extra>
            <div class="mt-1">
              {{ $gettext('To use the snippet, add this directive to a server or location block:') }}
              <ATypographyText code copyable>
                {{ include }}
              </ATypographyText>
            </div>
          </template>
        </AFormItem>

        <AFormItem v-if="isEditing" :label="$gettext('Used By')">
          <SnippetUsage
            v-if="loaded?.used_by.length"
            :used-by="loaded.used_by"
            :limit="8"
          />
          <span v-else class="text-gray-500 dark:text-gray-400">
            {{ $gettext('No configuration includes this snippet yet.') }}
          </span>
        </AFormItem>
      </AForm>
    </ASpin>

    <template #footer>
      <AFlex justify="end" gap="small">
        <AButton @click="open = false">
          {{ $gettext('Cancel') }}
        </AButton>
        <AButton
          type="primary"
          :loading="isSaving"
          :disabled="!isEditing && !form.baseName.trim()"
          @click="save"
        >
          {{ $gettext('Save') }}
        </AButton>
      </AFlex>
    </template>
  </ADrawer>
</template>
