<script setup lang="ts">
const thisYear = computed(() => new Date().getFullYear())
</script>

<template>
  <div class="footer center">
    Copyright © 2020 - {{ thisYear }} Nginx UI
  </div>
</template>

<style scoped>
.footer {

}
</style>
