<script setup lang="ts">
defineProps<{
  show: boolean
}>()
</script>

<template>
  <AAlert
    v-if="show"
    type="warning"
    :message="$gettext('Installation is not allowed after 10 minutes of system startup, please restart the Nginx UI.')"
    show-icon
    style="margin-bottom: 20px;"
  />
</template>
