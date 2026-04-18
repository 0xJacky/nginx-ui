<script setup lang="ts">
import settings, { PROTECTED_VALUE_PLACEHOLDER } from '@/api/settings'
import { use2FAModal } from '@/components/TwoFA'

const props = defineProps<{
  path: string
  placeholder?: string
}>()

const model = defineModel<string>({ required: true })

const { message } = useGlobalApp()
const twoFAModal = use2FAModal()
const revealedValue = ref('')
const show = ref(false)
const isLoading = ref(false)

watch(model, value => {
  if (value === PROTECTED_VALUE_PLACEHOLDER) {
    revealedValue.value = ''
    show.value = false
  }
})

const displayValue = computed(() => {
  if (show.value)
    return revealedValue.value || model.value

  if (model.value === PROTECTED_VALUE_PLACEHOLDER)
    return 'Sensitive value hidden'

  return model.value
})

async function ensureRevealedValue() {
  if (revealedValue.value)
    return revealedValue.value

  isLoading.value = true
  try {
    await twoFAModal.open()
    const { value } = await settings.get_protected_value(props.path)
    revealedValue.value = value
    model.value = value
    return value
  }
  finally {
    isLoading.value = false
  }
}

async function toggleShow() {
  if (!show.value)
    await ensureRevealedValue()

  show.value = !show.value
}

async function copyValue() {
  const value = show.value ? model.value : await ensureRevealedValue()
  await navigator.clipboard.writeText(value)
  message.success($gettext('Copied'))
}

function updateValue(value: string) {
  if (!show.value)
    return

  model.value = value
}
</script>

<template>
  <div class="sensitive-input-shell" :class="{ 'is-protected': !show }">
    <AInput
      :value="displayValue"
      :readonly="!show"
      :type="show ? 'text' : 'text'"
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
  </div>
</template>

<style scoped lang="less">
.sensitive-input-shell {
  position: relative;
  transition: filter 0.2s ease;
  border-radius: 10px;

  &.is-protected {
    :deep(.ant-input) {
      color: transparent;
      text-shadow: 0 0 8px rgba(15, 23, 42, 0.72);
      user-select: none;
      cursor: not-allowed;
      caret-color: transparent;
    }
    :deep(.ant-input-affix-wrapper) {
      background: rgba(148, 163, 184, 0.10);
      border-color: rgba(148, 163, 184, 0.32);
    }
  }
}

.dark .sensitive-input-shell {
  &.is-protected {
    :deep(.ant-input) {
      text-shadow: 0 0 8px rgba(226, 232, 240, 0.75);
    }
  }
}

@media (prefers-color-scheme: dark) {
  .sensitive-input-shell {
    &.is-protected {
      :deep(.ant-input) {
        text-shadow: 0 0 8px rgba(226, 232, 240, 0.75);
      }
    }
  }
}
</style>
