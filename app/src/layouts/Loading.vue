<script setup lang="ts">
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
          <AIcon
            spin
            style="font-size: 30px"
            type="loading"
          />
        </template>
      </ASpin>
    </div>
  </div>
</template>

<style scoped>

</style>
