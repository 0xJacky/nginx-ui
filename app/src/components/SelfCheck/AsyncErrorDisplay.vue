<script setup lang="ts">
import type { CosyError } from '@/lib/http/types'
import { translateError } from '@/lib/http/error'

const props = defineProps<{
  error: CosyError
  status: 'warning' | 'error'
}>()

const translatedError = await translateError(props.error)
</script>

<template>
  <ATag :color="status === 'warning' ? 'warning' : 'error'">
    {{ translatedError }}
  </ATag>
</template>
