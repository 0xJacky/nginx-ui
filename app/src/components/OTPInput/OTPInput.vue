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
    input-classes="otp-input"
    :num-inputs="6"
    input-type="numeric"
    should-auto-focus
    should-focus-order
    @on-complete="onComplete"
  />
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
:deep(.otp-input) {
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
}
</style>
