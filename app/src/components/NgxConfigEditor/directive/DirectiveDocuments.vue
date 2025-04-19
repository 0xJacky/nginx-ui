<script setup lang="ts">
import { useDirectiveStore } from './store'

const props = defineProps<{
  directive: string
}>()

const { nginxDirectivesDocsMap } = storeToRefs(useDirectiveStore())
</script>

<template>
  <AFormItem
    v-if="nginxDirectivesDocsMap?.[props.directive]"
    class="mb-0"
    :label="
      $ngettext('Document', 'Documents',
                nginxDirectivesDocsMap[props.directive].links.length)"
  >
    <div v-for="(link, idx) in nginxDirectivesDocsMap[props.directive]?.links" :key="idx" class="mb-2">
      <a :href="link">
        {{ link }}
      </a>
    </div>
  </AFormItem>
</template>

<style scoped lang="less">

</style>
