<script setup lang="ts">
import { LoadingOutlined } from '@antdv-next/icons'

defineProps<{
  loading: boolean | string
}>()
</script>

<template>
  <div
    v-show="loading"
    class="loading"
  >
    <div class="wrapper center">
      <ASpin>
        <template #indicator>
          <LoadingOutlined
            spin
            style="font-size: 30px"
          />
        </template>
      </ASpin>
    </div>
  </div>
</template>

<style scoped>

</style>
