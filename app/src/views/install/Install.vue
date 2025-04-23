<script setup lang="ts">
import InstallView from './components/InstallView.vue'

const route = useRoute()
</script>

<template>
  <InstallView :key="route.fullPath" />
</template>
