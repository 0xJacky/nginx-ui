<script setup lang="ts">
import settings, { PROTECTED_VALUE_PLACEHOLDER } from '@/api/settings'
import { use2FAModal } from '@/components/TwoFA'

const props = defineProps<{
  value: string
  path: string
}>()

const { message } = useGlobalApp()
const twoFAModal = use2FAModal()
const show = ref(false)
const isLoading = ref(false)
const revealedValue = ref('')

function maskText(text: string) {
  if (!text)
    return '*********'

  if (text.length <= 10)
    return '*********'

  const start = text.substring(0, Math.floor((text.length - 10) / 2))
  const end = text.substring(start.length + 10)

  return `${start}**********${end}`
}

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

const displayString = computed(() => {
  if (show.value)
    return revealedValue.value

  if (!revealedValue.value && props.value === PROTECTED_VALUE_PLACEHOLDER)
    return 'Sensitive value hidden'

  return revealedValue.value || props.value
})

const maskedString = computed(() => {
  if (show.value)
    return revealedValue.value

  return maskText(displayString.value)
})

async function toggleShow() {
  if (!show.value)
    await ensureRevealedValue()
  show.value = !show.value
}

async function copyValue() {
  const value = await ensureRevealedValue()
  await navigator.clipboard.writeText(value)
  message.success($gettext('Copied'))
}
</script>

<template>
  <div class="sensitive-row">
    <span
      class="mr-2 sensitive-value"
      :class="{ 'is-protected': !show }"
    >
      <span
        v-if="show"
        class="sensitive-value__text"
      >
        {{ displayString }}
      </span>
      <template v-else>
        <span class="sensitive-value__text sensitive-value__text--sizer">
          {{ maskedString }}
        </span>
        <span class="sensitive-value__blurred" aria-hidden="true">
          {{ maskedString }}
        </span>
      </template>
    </span>
    <a
      class="mr-2"
      @click="copyValue"
    >
      {{ $gettext('Copy') }}
    </a>
    <a @click="toggleShow">{{ show ? $gettext('Hide') : isLoading ? $gettext('Loading...') : $gettext('Show') }}</a>
  </div>
</template>

<style scoped lang="less">
.sensitive-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.sensitive-value {
  position: relative;
  display: inline-flex;
  align-items: center;
  min-height: 32px;
  max-width: min(100%, 420px);
  width: min(100%, 420px);
  padding: 6px 12px;
  border-radius: 6px;
  background: rgba(148, 163, 184, 0.06);
  border: 1px solid rgba(148, 163, 184, 0.20);
  overflow: hidden;
  transition: border-color 0.2s ease, background 0.2s ease;

  &__text {
    display: inline-block;
    max-width: 100%;
    width: 100%;
    font-family: ui-monospace, SFMono-Regular, SFMono-Regular, Menlo, Monaco, Consolas, Liberation Mono, monospace;
    font-size: 13px;
    line-height: 1.4;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    transition: filter 0.2s ease, opacity 0.2s ease;
  }

  &__text--sizer {
    visibility: hidden;
  }

  &__blurred {
    position: absolute;
    inset: 0;
    display: block;
    width: auto;
    padding: 6px 12px;
    font-family: ui-monospace, SFMono-Regular, SFMono-Regular, Menlo, Monaco, Consolas, Liberation Mono, monospace;
    font-size: 13px;
    line-height: 1.4;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    color: transparent;
    text-shadow: 0 0 8px rgba(15, 23, 42, 0.72);
    user-select: none;
    pointer-events: none;
  }

  &.is-protected {
    background: rgba(148, 163, 184, 0.11);
    border-color: rgba(148, 163, 184, 0.24);
  }
}

.dark .sensitive-value {
  background: rgba(148, 163, 184, 0.08);
  border-color: rgba(148, 163, 184, 0.16);

  &.is-protected {
    background: rgba(148, 163, 184, 0.12);
    border-color: rgba(148, 163, 184, 0.20);
  }

  .sensitive-value__blurred {
    text-shadow: 0 0 8px rgba(226, 232, 240, 0.75);
  }
}

@media (prefers-color-scheme: dark) {
  .sensitive-value {
    background: rgba(148, 163, 184, 0.08);
    border-color: rgba(148, 163, 184, 0.16);

    &.is-protected {
      background: rgba(148, 163, 184, 0.12);
      border-color: rgba(148, 163, 184, 0.20);
    }
  }

  .sensitive-value__blurred {
    text-shadow: 0 0 8px rgba(226, 232, 240, 0.75);
  }
}
</style>
