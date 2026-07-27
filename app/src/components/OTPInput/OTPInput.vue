<script setup lang="ts">
import VOtpInput from 'vue3-otp-input'

const emit = defineEmits(['onComplete'])

const data = defineModel<string>({
  default: '',
})

// eslint-disable-next-line vue/require-typed-ref
const refOtp = ref()

function onComplete(value: string) {
  emit('onComplete', value)
}

function clearInput() {
  refOtp.value?.clearInput()
}

defineExpose({
  clearInput,
})
</script>

<template>
  <VOtpInput
    ref="refOtp"
    v-model:value="data"
    class="otp-input-wrapper"
    input-classes="otp-input"
    :num-inputs="6"
    input-mode="numeric"
    should-auto-focus
    should-focus-order
    @on-complete="onComplete"
  />
</template>

<style lang="less">
// vue3-otp-input wraps every input in its own div, so the layout rules have to
// reach that div. Kept unscoped for that reason and namespaced by the wrapper.
.otp-input-wrapper {
  display: flex;
  flex-wrap: nowrap;
  gap: 8px;
  justify-content: center;
  width: 100%;
  max-width: 100%;

  // The generated per input wrapper. It is the real flex item.
  > div {
    flex: 1 1 0;
    min-width: 0;
    max-width: 40px;
  }

  .otp-input {
    width: 100%;
    min-width: 0;
    padding: 0;
    margin: 0;
    aspect-ratio: 1;
    font-size: 18px;
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
  }

  @media (max-width: 600px) {
    gap: 6px;
  }
}

.dark .otp-input-wrapper .otp-input {
  border: 1px solid rgba(255, 255, 255, 0.2);

  &:focus {
    border: 2px solid #1677ff;
  }
}
</style>
