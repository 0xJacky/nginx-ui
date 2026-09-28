<script setup lang="ts">
import type { SelectProps } from 'antdv-next'
import type {
  AccessFallback,
  AccessList,
  AccessListPayload,
  AccessListUsage,
  AccessListWarning,
  AccessReference,
  AccessRuleType,
} from '@/api/access_list'
import { DeleteOutlined, HolderOutlined, PlusOutlined } from '@antdv-next/icons'
import { watchDebounced } from '@vueuse/core'
import Draggable from 'vuedraggable'
import accessList from '@/api/access_list'
import CodeEditor from '@/components/CodeEditor'
import { translateError } from '@/lib/http/error'
import { normalizeHttpError } from '@/lib/http/normalizeError'
import { warningText } from '../warnings'

const props = defineProps<{
  // List to edit; undefined to create a new one.
  id?: number
  // Every stored list, for the reference rule select.
  lists: AccessList[]
}>()

const emit = defineEmits<{
  saved: [list: AccessList]
}>()

const open = defineModel<boolean>('open', { default: false })

const { message } = App.useApp()
const router = useRouter()

interface RuleRow {
  key: number
  type: AccessRuleType
  value: string
  ref_id?: number
  note: string
}

interface FormState {
  name: string
  slug: string
  fallback: AccessFallback
  rules: RuleRow[]
}

let rowSeed = 0

function createRule(type: AccessRuleType = 'allow'): RuleRow {
  return { key: rowSeed++, type, value: '', note: '' }
}

function createForm(): FormState {
  return { name: '', slug: '', fallback: 'deny', rules: [createRule()] }
}

const form = reactive<FormState>(createForm())
const isSlugTouched = ref(false)
const isLoading = ref(false)
const isSaving = ref(false)
const previewContent = ref('')
const previewError = ref('')
const warnings = ref<AccessListWarning[]>([])
const usage = ref<AccessListUsage>()

const isEditing = computed(() => props.id !== undefined)

const typeOptions = computed<SelectProps['options']>(() => [
  { label: $gettext('Allow'), value: 'allow' },
  { label: $gettext('Deny'), value: 'deny' },
  { label: $gettext('Use list'), value: 'ref' },
])

const fallbackOptions = computed<SelectProps['options']>(() => [
  { label: $gettext('Deny (403)'), value: 'deny' },
  { label: $gettext('Allow'), value: 'allow' },
])

const referenceOptions = computed<SelectProps['options']>(() => props.lists
  .filter(l => l.id !== props.id)
  .map(l => ({ label: l.name, value: l.id })))

function slugify(name: string) {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 63)
}

watch(() => form.name, name => {
  if (!isEditing.value && !isSlugTouched.value)
    form.slug = slugify(name)
})

function toPayload(): AccessListPayload {
  return {
    id: props.id,
    name: form.name.trim(),
    slug: form.slug.trim(),
    fallback: form.fallback,
    // A row that was added but not filled in yet is not a rule.
    rules: form.rules
      .filter(rule => rule.type === 'ref' ? !!rule.ref_id : !!rule.value.trim())
      .map(rule => rule.type === 'ref'
        ? { type: 'ref', ref_id: rule.ref_id, note: rule.note.trim() }
        : { type: rule.type, value: rule.value.trim(), note: rule.note.trim() }),
  }
}

async function load() {
  Object.assign(form, createForm())
  isSlugTouched.value = false
  previewContent.value = ''
  previewError.value = ''
  warnings.value = []
  usage.value = undefined

  if (props.id === undefined)
    return

  isLoading.value = true
  try {
    const [detail, used] = await Promise.all([
      accessList.getItem(props.id),
      accessList.getUsage(props.id),
    ])
    Object.assign(form, {
      name: detail.name,
      slug: detail.slug,
      fallback: detail.fallback,
      rules: detail.rules.map(rule => ({
        key: rowSeed++,
        type: rule.type,
        value: rule.value ?? '',
        ref_id: rule.ref_id || undefined,
        note: rule.note ?? '',
      })),
    })
    usage.value = used
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

async function refreshPreview() {
  if (!open.value || isLoading.value)
    return
  const payload = toPayload()
  if (!payload.name) {
    previewContent.value = ''
    previewError.value = ''
    warnings.value = []
    return
  }
  try {
    const res = await accessList.preview(payload)
    previewContent.value = res.content
    warnings.value = res.warnings ?? []
    previewError.value = ''
  }
  catch (error) {
    previewError.value = await translateError(normalizeHttpError(error))
  }
}

watchDebounced(form, refreshPreview, { debounce: 300, deep: true })

function addRule(type: AccessRuleType) {
  form.rules.push(createRule(type))
}

function removeRule(index: number) {
  form.rules.splice(index, 1)
}

function onTypeChange(rule: RuleRow) {
  if (rule.type === 'ref') {
    rule.value = ''
    rule.ref_id ??= referenceOptions.value?.[0]?.value as number | undefined
  }
  else {
    rule.ref_id = undefined
  }
}

function referenceLabel(ref: AccessReference) {
  const kind = ref.kind === 'site' ? $gettext('Site') : $gettext('Stream')
  return ref.location ? `${kind} ${ref.name} · ${ref.location}` : `${kind} ${ref.name}`
}

function openReference(ref: AccessReference) {
  router.push(ref.kind === 'site' ? `/sites/${encodeURIComponent(ref.name)}` : `/streams/${encodeURIComponent(ref.name)}`)
}

async function save() {
  isSaving.value = true
  try {
    const payload = toPayload()
    const saved = isEditing.value
      ? await accessList.updateItem(props.id!, payload)
      : await accessList.createItem(payload)
    message.success($gettext('Access list %{name} saved', { name: saved.name }))
    emit('saved', saved)
    open.value = false
  }
  catch {
    // The request layer already shows the error, including nginx -t output.
  }
  finally {
    isSaving.value = false
  }
}
</script>

<template>
  <ADrawer
    v-model:open="open"
    :title="isEditing ? $gettext('Edit Access List %{name}', { name: form.name }) : $gettext('Create Access List')"
    :size="760"
    destroy-on-hidden
    data-testid="access-list-editor"
  >
    <ASpin :spinning="isLoading">
      <AForm layout="vertical">
        <AFlex gap="middle" wrap>
          <AFormItem :label="$gettext('Name')" required class="min-w-60 flex-1">
            <AInput
              v-model:value="form.name"
              :maxlength="100"
              placeholder="LAN"
              data-testid="access-list-name"
            />
          </AFormItem>
          <AFormItem
            :label="$gettext('Identifier')"
            required
            class="min-w-60 flex-1"
            :extra="isEditing
              ? $gettext('Fixed after creation, so sites keep working when the list is renamed.')
              : $gettext('Names the file; lowercase letters, digits and hyphens.')"
          >
            <AInput
              v-model:value="form.slug"
              class="font-mono"
              :disabled="isEditing"
              placeholder="lan"
              data-testid="access-list-slug"
              @input="isSlugTouched = true"
            />
          </AFormItem>
        </AFlex>

        <AFormItem
          :label="$gettext('Rules')"
          :extra="$gettext('Checked from top to bottom; the first rule that matches decides. A list rule expands the rules of that list in place.')"
        >
          <div class="flex flex-col gap-2">
            <div class="hidden md:grid rule-grid gap-2 text-xs text-gray-500 dark:text-gray-400">
              <span />
              <span>{{ $gettext('Action') }}</span>
              <span>{{ $gettext('Address or list') }}</span>
              <span>{{ $pgettext('Access list rule', 'Note') }}</span>
              <span />
            </div>
            <Draggable
              :list="form.rules"
              item-key="key"
              handle=".rule-handle"
              class="flex flex-col gap-2"
            >
              <template #item="{ element: rule, index }">
                <div class="grid rule-grid gap-2 items-center" data-testid="access-list-rule">
                  <HolderOutlined class="rule-handle cursor-move text-gray-400" />
                  <ASelect
                    v-model:value="rule.type"
                    :options="typeOptions"
                    :aria-label="$gettext('Action')"
                    @change="onTypeChange(rule)"
                  />
                  <ASelect
                    v-if="rule.type === 'ref'"
                    v-model:value="rule.ref_id"
                    :options="referenceOptions"
                    :placeholder="$gettext('Select a list')"
                    :aria-label="$gettext('Address or list')"
                  />
                  <AInput
                    v-else
                    v-model:value="rule.value"
                    class="font-mono"
                    placeholder="192.168.1.0/24"
                    :aria-label="$gettext('Address or list')"
                    data-testid="access-list-rule-value"
                  />
                  <AInput
                    v-model:value="rule.note"
                    :placeholder="$pgettext('Access list rule', 'Note')"
                    :aria-label="$pgettext('Access list rule', 'Note')"
                  />
                  <AButton
                    type="text"
                    danger
                    :aria-label="$gettext('Remove')"
                    @click="removeRule(index)"
                  >
                    <template #icon>
                      <DeleteOutlined />
                    </template>
                  </AButton>
                </div>
              </template>
            </Draggable>
            <AFlex gap="small" wrap>
              <AButton type="dashed" data-testid="access-list-add-allow" @click="addRule('allow')">
                <template #icon>
                  <PlusOutlined />
                </template>
                {{ $gettext('Allow') }}
              </AButton>
              <AButton type="dashed" @click="addRule('deny')">
                <template #icon>
                  <PlusOutlined />
                </template>
                {{ $gettext('Deny') }}
              </AButton>
              <AButton
                type="dashed"
                :disabled="(referenceOptions?.length ?? 0) === 0"
                @click="addRule('ref')"
              >
                <template #icon>
                  <PlusOutlined />
                </template>
                {{ $gettext('Use list') }}
              </AButton>
            </AFlex>
          </div>
        </AFormItem>

        <AFormItem
          :label="$gettext('Everything else')"
          :extra="$gettext('Applies to requests no rule matched. Lists used by this list do not add their own.')"
        >
          <ASelect
            v-model:value="form.fallback"
            class="w-full max-w-100"
            :options="fallbackOptions"
            data-testid="access-list-fallback"
          />
        </AFormItem>

        <AFormItem v-if="warnings.length > 0 || previewError">
          <div class="flex flex-col gap-2">
            <AAlert
              v-if="previewError"
              type="error"
              show-icon
              :title="previewError"
              data-testid="access-list-preview-error"
            />
            <AAlert
              v-for="warning in warnings"
              :key="`${warning.code}-${warning.rule}`"
              type="warning"
              show-icon
              :title="warningText(warning)"
            />
          </div>
        </AFormItem>

        <AFormItem
          :label="$gettext('File Preview')"
          :extra="form.slug ? `nginx-ui/access/${form.slug}.conf` : ''"
        >
          <CodeEditor
            :content="previewContent"
            readonly
            default-height="200px"
          />
        </AFormItem>

        <AFormItem v-if="isEditing" :label="$gettext('Used By')">
          <div
            v-if="usage && (usage.lists.length > 0 || usage.references.length > 0)"
            class="flex flex-wrap gap-2"
          >
            <ATag v-for="l in usage.lists" :key="`list-${l.id}`">
              {{ $gettext('List %{name}', { name: l.name }) }}
            </ATag>
            <ATag
              v-for="(ref, i) in usage.references"
              :key="`ref-${i}`"
              color="blue"
              class="cursor-pointer"
              @click="openReference(ref)"
            >
              {{ referenceLabel(ref) }}
            </ATag>
          </div>
          <span v-else class="text-gray-500 dark:text-gray-400">
            {{ $gettext('Nothing uses this list yet.') }}
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
          data-testid="access-list-save"
          @click="save"
        >
          {{ $gettext('Save') }}
        </AButton>
      </AFlex>
    </template>
  </ADrawer>
</template>

<style scoped>
/* Narrow screens: handle, action and delete on the first line, the address
   and the note below them. */
.rule-grid {
  grid-template-columns: 16px 1fr 32px;
}

.rule-grid > :nth-child(3),
.rule-grid > :nth-child(4) {
  grid-column: 2 / 4;
}

.rule-grid > :nth-child(5) {
  grid-column: 3;
  grid-row: 1;
}

@media (min-width: 768px) {
  .rule-grid {
    grid-template-columns: 16px 110px minmax(0, 1.4fr) minmax(0, 1fr) 32px;
  }

  .rule-grid > :nth-child(3),
  .rule-grid > :nth-child(4),
  .rule-grid > :nth-child(5) {
    grid-column: auto;
    grid-row: auto;
  }
}
</style>
