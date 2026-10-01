<script setup lang="ts">
import type { VariableRow } from './variables'
import type { Snippet } from '@/api/snippet'
import { useEventListener } from '@vueuse/core'
import snippet from '@/api/snippet'
import BaseEditor from '@/components/BaseEditor'
import FooterToolBar from '@/components/FooterToolbar'
import LocalizedInput from './components/LocalizedInput.vue'
import SnippetCode from './components/SnippetCode.vue'
import SnippetPreview from './components/SnippetPreview.vue'
import SnippetUsage from './components/SnippetUsage.vue'
import SnippetVariables from './components/SnippetVariables.vue'
import { localize, useSnippetDescription } from './description'
import { referencedKeys } from './template'
import { hasVariableProblems, newVariable, toRows, toVariables } from './variables'

const route = useRoute()
const router = useRouter()
const { message, modal } = App.useApp()
const { current: language } = useSnippetDescription()

// File of the snippet to edit; undefined to create a new one.
const file = computed(() => route.name === 'Edit Snippet' ? String(route.params.file) : undefined)

interface FormState {
  baseName: string
  names: Record<string, string>
  description: Record<string, string>
  author: string
  variables: VariableRow[]
  content: string
}

function createForm(): FormState {
  return { baseName: '', names: {}, description: {}, author: '', variables: [], content: '' }
}

const form = reactive<FormState>(createForm())
const loaded = ref<Snippet>()
const isBaseNameTouched = ref(false)
const isLoading = ref(false)
const isSaving = ref(false)
// The form as loaded or saved, to tell whether it has unsaved changes.
const pristine = ref('')
const activeKey = ref<string>()

const isEditing = computed(() => file.value !== undefined)
const fileName = computed(() => `${form.baseName.trim()}.conf`)
// Include directives name the file, so only a snippet nothing includes can
// move to another one.
const canRename = computed(() => isEditing.value && loaded.value?.used_by.length === 0)
const displayName = computed(() => localize(form.names, language.value))
const hasProblems = computed(() => hasVariableProblems(form.variables))
const isDirty = computed(() => !isLoading.value && JSON.stringify(form) !== pristine.value)
const referenced = computed(() => referencedKeys(form.content))
const undeclared = computed(() => [...referenced.value].filter(key => !form.variables.some(v => v.key.trim() === key)))
const isIncludable = computed(() => form.variables.length === 0)
// A snippet with variables is filled in by the site editor, never included.
const include = computed(() => form.baseName.trim() && isIncludable.value ? `include snippets/${fileName.value};` : '')

function toBaseName(name: string) {
  return name.toLowerCase().replace(/[^a-z0-9._-]+/g, '-').replace(/^[^a-z0-9]+|-+$/g, '').slice(0, 64)
}

watch(displayName, name => {
  if (!isEditing.value && !isBaseNameTouched.value)
    form.baseName = toBaseName(name)
})

async function load() {
  Object.assign(form, createForm())
  loaded.value = undefined
  isBaseNameTouched.value = false
  activeKey.value = undefined

  if (file.value === undefined) {
    pristine.value = JSON.stringify(form)
    // A copy of a built-in template starts from it; until it is saved it
    // counts as unsaved.
    if (typeof route.query.from === 'string')
      await copyBuiltin(route.query.from)
    return
  }

  isLoading.value = true
  try {
    const detail = await snippet.get(file.value)
    loaded.value = detail
    Object.assign(form, {
      baseName: detail.file.replace(/\.conf$/, ''),
      names: { ...detail.name_i18n },
      description: { ...detail.description },
      author: detail.author,
      variables: toRows(detail.variables),
      content: detail.content ?? '',
    })
    pristine.value = JSON.stringify(form)
  }
  catch {
    router.replace('/sites/snippets')
  }
  finally {
    isLoading.value = false
  }
}

async function copyBuiltin(name: string) {
  isLoading.value = true
  try {
    const source = await snippet.getBuiltin(name)
    Object.assign(form, {
      baseName: source.filename.replace(/\.conf$/, ''),
      names: source.name ? { en: source.name } : {},
      description: { ...source.description },
      author: source.author,
      variables: toRows(source.variables),
      content: source.content ?? '',
    })
    isBaseNameTouched.value = true
  }
  finally {
    isLoading.value = false
  }
}

watch(file, load, { immediate: true })

function addVariable(row: VariableRow) {
  form.variables.push(row)
}

function declare(key: string) {
  addVariable(newVariable(key))
  activeKey.value = key
}

async function save() {
  isSaving.value = true
  try {
    const payload = {
      name_i18n: form.names,
      description: form.description,
      author: form.author.trim(),
      variables: toVariables(form.variables),
      content: form.content,
    }
    const saved = isEditing.value
      ? await snippet.update(file.value!, { ...payload, file: fileName.value !== file.value ? fileName.value : undefined })
      : await snippet.create({ ...payload, file: fileName.value })
    message.success($gettext('Snippet %{name} saved', { name: localize(saved.name_i18n, language.value) || saved.name }))
    pristine.value = JSON.stringify(form)
    if (saved.file !== file.value)
      router.replace(`/sites/snippets/${encodeURIComponent(saved.file)}`)
    else
      loaded.value = saved
  }
  catch {
    // The request layer already shows the error, including the nginx -t output.
  }
  finally {
    isSaving.value = false
  }
}

function confirmDiscard() {
  return new Promise<boolean>(resolve => {
    modal.confirm({
      title: $gettext('Discard the unsaved changes?'),
      okText: $gettext('Discard'),
      okButtonProps: { danger: true },
      cancelText: $gettext('Keep Editing'),
      onOk: () => resolve(true),
      onCancel: () => resolve(false),
    })
  })
}

onBeforeRouteLeave(async () => {
  if (isDirty.value && !isSaving.value)
    return confirmDiscard()
})

// The browser asks before closing a page with unsaved changes.
useEventListener(window, 'beforeunload', (event: BeforeUnloadEvent) => {
  if (isDirty.value)
    event.preventDefault()
})
</script>

<template>
  <BaseEditor :loading="isLoading">
    <template #left>
      <ACard
        variant="borderless"
        class="snippet-card"
      >
        <template #title>
          <div class="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
            <span class="truncate">
              {{ isEditing ? $gettext('Edit Snippet %{name}', { name: displayName || fileName }) : $gettext('Create Snippet') }}
            </span>
            <ATooltip
              :title="isIncludable
                ? $gettext('Sites can include this snippet and follow its later changes.')
                : $gettext('This snippet has variables, so sites insert a filled in copy from the config template panel.')"
            >
              <ATag
                :color="isIncludable ? 'green' : 'gold'"
                :bordered="false"
                class="m-0 font-normal"
              >
                {{ isIncludable ? $gettext('Includable') : $gettext('Insert Only') }}
              </ATag>
            </ATooltip>
            <span
              v-if="form.baseName.trim()"
              class="hint font-mono text-xs font-normal max-md:hidden"
            >snippets/{{ fileName }}</span>
          </div>
        </template>
        <div class="flex flex-col gap-2">
          <SnippetCode
            v-model="form.content"
            v-model:active="activeKey"
            :rows="form.variables"
            :lines="[8, 40]"
            @add-variable="addVariable"
          />
          <AAlert
            v-if="undeclared.length > 0"
            type="warning"
            show-icon
            class="lg:hidden"
          >
            <template #title>
              {{ $gettext('Used in the content, not declared') }}
              <AButton
                v-for="key in undeclared"
                :key="key"
                size="small"
                type="link"
                class="font-mono"
                @click="declare(key)"
              >
                .{{ key }}
              </AButton>
            </template>
          </AAlert>
        </div>
        <section class="mt-6">
          <div class="mb-3 flex flex-wrap items-baseline gap-x-2">
            <span class="text-base font-medium">{{ include ? $gettext('Use') : $gettext('Preview') }}</span>
            <span class="hint text-sm">
              {{ include
                ? $gettext('Add this directive to a server or location block. Sites that include the snippet follow its later changes.')
                : $gettext('Updates as the snippet changes.') }}
            </span>
          </div>
          <SnippetPreview
            :rows="form.variables"
            :content="form.content"
            :include="include"
          />
        </section>
      </ACard>
    </template>

    <template #right>
      <ACard
        variant="borderless"
        :title="$gettext('Details')"
        :styles="{ body: { maxHeight: 'calc(100vh - 220px)', overflowY: 'auto' } }"
      >
        <div class="flex flex-col gap-4">
          <label class="flex flex-col gap-1">
            <span>{{ $gettext('Name') }}</span>
            <LocalizedInput
              v-model="form.names"
              :maxlength="100"
              :placeholder="$gettext('Static file cache')"
            />
          </label>
          <label class="flex flex-col gap-1">
            <span><span class="required">*</span>{{ $gettext('File Name') }}</span>
            <AInput
              v-model:value="form.baseName"
              class="font-mono"
              :disabled="isEditing && !canRename"
              placeholder="static-cache"
              @input="isBaseNameTouched = true"
            >
              <template #suffix>
                <span class="hint font-mono">.conf</span>
              </template>
            </AInput>
            <span class="hint text-xs">
              {{ isEditing && !canRename
                ? $gettext('Fixed while configurations include the snippet, so they keep working.')
                : $gettext('Letters, digits, dots, dashes and underscores.') }}
            </span>
          </label>
          <label class="flex flex-col gap-1">
            <span>{{ $gettext('Description') }}</span>
            <LocalizedInput
              v-model="form.description"
              :maxlength="200"
            />
          </label>
          <label class="flex flex-col gap-1">
            <span>{{ $gettext('Author') }}</span>
            <AInput
              v-model:value="form.author"
              :maxlength="100"
            />
          </label>
          <div class="flex flex-col gap-2">
            <span>{{ $gettext('Variables') }}</span>
            <SnippetVariables
              v-model="form.variables"
              v-model:active="activeKey"
              :referenced="referenced"
            />
            <span class="hint text-xs">
              {{ $gettext('Variables are filled in when the snippet is inserted from the config template panel of the site editor. A snippet with variables can only be inserted, not included.') }}
            </span>
          </div>
          <div
            v-if="isEditing"
            class="flex flex-col gap-1"
          >
            <span>{{ $gettext('Used By') }}</span>
            <SnippetUsage
              v-if="loaded?.used_by.length"
              :used-by="loaded.used_by"
              :limit="8"
            />
            <span
              v-else
              class="hint"
            >
              {{ $gettext('No configuration includes this snippet yet.') }}
            </span>
          </div>
        </div>
      </ACard>
    </template>
  </BaseEditor>

  <FooterToolBar>
    <ASpace>
      <span
        v-if="isDirty"
        class="hint text-sm"
      >{{ $gettext('Unsaved changes') }}</span>
      <AButton @click="router.push('/sites/snippets')">
        {{ $gettext('Back') }}
      </AButton>
      <AButton
        type="primary"
        :loading="isSaving"
        :disabled="!form.baseName.trim() || hasProblems"
        @click="save"
      >
        {{ $gettext('Save') }}
      </AButton>
    </ASpace>
  </FooterToolBar>
</template>

<style scoped lang="less">
.required {
  margin-inline-end: 4px;
  color: var(--ant-color-error);
}

.hint {
  color: var(--ant-color-text-secondary);
}
</style>
