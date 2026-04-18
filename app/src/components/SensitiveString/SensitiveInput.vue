<script setup lang="ts">
import settings, { PROTECTED_VALUE_PLACEHOLDER } from '@/api/settings'
import { use2FAModal } from '@/components/TwoFA'

const props = defineProps<{
  modelValue: string
  path: string
  placeholder?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const { message } = useGlobalApp()
const twoFAModal = use2FAModal()
const inputValue = ref(props.modelValue)
const revealedValue = ref('')
const isDirty = ref(false)
const show = ref(false)
const isLoading = ref(false)

watch(() => props.modelValue, value => {
  if (value === PROTECTED_VALUE_PLACEHOLDER) {
    inputValue.value = value
    revealedValue.value = ''
    isDirty.value = false
    show.value = false
    return
  }

  if (!isDirty.value)
    inputValue.value = value
})

const displayValue = computed(() => {
  if (isDirty.value)
    return inputValue.value

  if (show.value)
    return revealedValue.value

  return props.modelValue
})

async function ensureRevealedValue() {
  if (revealedValue.value)
    return revealedValue.value

  isLoading.value = true
  try {
    await twoFAModal.open()
    const { value } = await settings.get_protected_value(props.path)
    revealedValue.value = value
    return value
  }
  finally {
    isLoading.value = false
  }
}

async function toggleShow() {
  if (!show.value && !isDirty.value)
    await ensureRevealedValue()

  show.value = !show.value
}

async function copyValue() {
  const value = isDirty.value ? inputValue.value : await ensureRevealedValue()
  await navigator.clipboard.writeText(value)
  message.success($gettext('Copied'))
}

function updateValue(value: string) {
  inputValue.value = value
  isDirty.value = true
  emit('update:modelValue', value)
}
</script>

<template>
  <AInput
    :value="displayValue"
    :type="show ? 'text' : 'password'"
    :placeholder="placeholder"
    @update:value="updateValue"
  >
    <template #suffix>
      <ASpace size="small">
        <a @click.prevent="copyValue">
          {{ $gettext('Copy') }}
        </a>
        <a @click.prevent="toggleShow">
          {{ show ? $gettext('Hide') : isLoading ? $gettext('Loading...') : $gettext('Show') }}
        </a>
      </ASpace>
    </template>
  </AInput>
</template>
