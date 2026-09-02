<script setup lang="ts">
import type { ComponentPublicInstance } from 'vue'

/**
 * Segmented one-time-code input.
 *
 * The markup is intentionally password-manager friendly, because this
 * component is also mounted dynamically inside the secure-session 2FA modal
 * where browser extensions have to re-discover the fields at runtime:
 *
 * - the boxes live inside a real `<form>` whose id/name/class contain `otp`,
 *   so extensions that only scan form contexts (or that re-scan a subtree
 *   when its class/style mutates, e.g. while a modal animates in) can find
 *   them;
 * - every box carries `autocomplete="one-time-code"`, `inputmode="numeric"`,
 *   `pattern="[0-9]*"` and `maxlength="1"`, the shape browsers and password
 *   managers recognise as a segmented TOTP field;
 * - ids and names are stable across mounts, so nothing depends on a random
 *   value generated at mount time;
 * - a value longer than one character is distributed over the boxes, which
 *   covers autofill implementations that write the whole code into a single
 *   box instead of filling the boxes one by one.
 */
const props = withDefaults(defineProps<{
  /** Number of digits of the one-time code. */
  length?: number
  /** Base id/name used for the form and its inputs. Must stay stable. */
  name?: string
  disabled?: boolean
  autoFocus?: boolean
}>(), {
  length: 6,
  name: 'otp',
  disabled: false,
  autoFocus: true,
})

const emit = defineEmits(['onComplete'])

const data = defineModel<string>({
  default: '',
})

/** Spreads a raw string over exactly `length` boxes, padding with empties. */
function toDigits(value: string): string[] {
  return Array.from({ length: props.length }, (_, index) => value[index] ?? '')
}

const digits = ref<string[]>(toDigits(''))
const inputRefs = ref<(HTMLInputElement | null)[]>([])

const inputIds = computed(() =>
  Array.from({ length: props.length }, (_, index) => (index === 0 ? props.name : `${props.name}-${index + 1}`)),
)

function setInputRef(el: Element | ComponentPublicInstance | null, index: number) {
  inputRefs.value[index] = (el as HTMLInputElement | null) ?? null
}

// Keeps the DOM in sync when a box keeps its previous value but the user (or a
// password manager) wrote something else into it: Vue only patches inputs whose
// bound value actually changed, so rejected characters would otherwise stay in
// the DOM. This runs synchronously because a leftover character would make the
// browser drop the next keystroke, `maxlength` being 1.
function syncInputElements() {
  inputRefs.value.forEach((el, index) => {
    const value = digits.value[index] ?? ''
    if (el && el.value !== value)
      el.value = value
  })
}

function focusInput(index: number) {
  const target = inputRefs.value[Math.min(Math.max(index, 0), props.length - 1)]

  target?.focus()
  target?.select()
}

function commit(next: string[]) {
  const previous = digits.value.join('')
  const value = next.join('')

  digits.value = next
  data.value = value
  syncInputElements()

  if (value !== previous && value.length === props.length)
    emit('onComplete', value)
}

/** Writes `chars` into consecutive boxes and returns the next free index. */
function fillFrom(chars: string, startIndex: number) {
  const next = [...digits.value]
  let cursor = startIndex

  for (const char of chars) {
    if (cursor >= props.length)
      break

    next[cursor] = char
    cursor += 1
  }

  commit(next)

  return cursor
}

function handleInput(event: Event, index: number) {
  const el = event.target as HTMLInputElement
  const previous = digits.value[index] ?? ''
  let incoming = el.value.replace(/\D/g, '')

  // A value that is exactly as long as the code is always treated as the
  // complete code and is spread from the first box, no matter which box
  // received it. This keeps autofill idempotent when a password manager
  // writes the whole code into several boxes in a row.
  if (incoming.length === props.length) {
    fillFrom(incoming, 0)
    focusInput(props.length - 1)
    return
  }

  // Typing into a box that already holds a digit appends to its value, so keep
  // only the characters that were added.
  if (previous && incoming.length > previous.length && incoming.startsWith(previous))
    incoming = incoming.slice(previous.length)

  if (!incoming) {
    const next = [...digits.value]

    next[index] = ''
    commit(next)
    return
  }

  focusInput(fillFrom(incoming, index))
}

function handlePaste(event: ClipboardEvent, index: number) {
  event.preventDefault()

  const chars = (event.clipboardData?.getData('text') ?? '').replace(/\D/g, '')
  if (!chars)
    return

  focusInput(fillFrom(chars, chars.length === props.length ? 0 : index))
}

function clearAt(index: number) {
  const next = [...digits.value]

  next[index] = ''
  commit(next)
}

function handleKeydown(event: KeyboardEvent, index: number) {
  if (event.ctrlKey || event.metaKey || event.altKey)
    return

  switch (event.key) {
    case 'Backspace':
      event.preventDefault()
      if (digits.value[index]) {
        clearAt(index)
        focusInput(index)
      }
      else if (index > 0) {
        clearAt(index - 1)
        focusInput(index - 1)
      }
      break
    case 'Delete':
      event.preventDefault()
      clearAt(index)
      break
    case 'ArrowLeft':
      event.preventDefault()
      focusInput(index - 1)
      break
    case 'ArrowRight':
      event.preventDefault()
      focusInput(index + 1)
      break
    default:
      break
  }
}

function handleFocus(event: FocusEvent) {
  (event.target as HTMLInputElement).select()
}

function clearInput() {
  commit(toDigits(''))
  focusInput(0)
}

// Accept values assigned from the outside (for example a reset by the parent).
watch(data, value => {
  const normalized = (value ?? '').replace(/\D/g, '')
  if (normalized === digits.value.join(''))
    return

  digits.value = toDigits(normalized)
  syncInputElements()
}, { immediate: true })

onMounted(() => {
  if (props.autoFocus)
    focusInput(0)
})

defineExpose({
  clearInput,
})
</script>

<template>
  <form
    :id="`${name}-form`"
    :name="`${name}-form`"
    class="otp-input-form"
    @submit.prevent
  >
    <input
      v-for="(id, index) in inputIds"
      :id="id"
      :key="id"
      :ref="el => setInputRef(el, index)"
      :name="id"
      :value="digits[index]"
      :disabled="disabled"
      :aria-label="$gettext('Digit %{position} of the one-time code', { position: `${index + 1}` })"
      class="otp-input"
      type="text"
      inputmode="numeric"
      pattern="[0-9]*"
      autocomplete="one-time-code"
      maxlength="1"
      @input="handleInput($event, index)"
      @keydown="handleKeydown($event, index)"
      @paste="handlePaste($event, index)"
      @focus="handleFocus"
    >
  </form>
</template>

<style lang="less">
.dark {
  .otp-input {
    border: 1px solid rgba(255, 255, 255, 0.2) !important;

    &:focus {
      outline: none;
      border: 2px solid #1677ff !important;
    }
  }
}
</style>

<style scoped lang="less">
.otp-input-form {
  display: flex;
  align-items: center;
  margin: 0;
}

.otp-input {
  width: 40px;
  height: 40px;
  padding: 5px;
  margin: 0 10px;
  font-size: 20px;
  border-radius: 4px;
  border: 1px solid rgba(0, 0, 0, 0.3);

  text-align: center;
  background-color: transparent;

  &:focus {
    outline: none;
    border: 2px solid #1677ff;
  }

  &::-webkit-inner-spin-button,
  &::-webkit-outer-spin-button {
    -webkit-appearance: none;
    margin: 0;
  }

  // Six fixed 60px columns overflow the 2FA modal on narrow screens, so let
  // the boxes share the available width there instead of the fixed size.
  @media (max-width: 600px) {
    flex: 1 1 0;
    width: auto;
    height: auto;
    min-width: 0;
    max-width: 40px;
    aspect-ratio: 1;
    margin: 0 4px;
    font-size: 18px;
  }
}
</style>
