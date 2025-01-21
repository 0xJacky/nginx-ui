<script setup lang="ts">
import type { DirectiveMap } from '@/api/ngx'

const props = defineProps<{
  directive: string
  nginxDirectivesMap?: DirectiveMap
}>()
</script>

<template>
  <AFormItem
    v-if="nginxDirectivesMap?.[props.directive]"
    class="mb-0"
    :label="
      $ngettext('Document', 'Documents',
                nginxDirectivesMap[props.directive].links.length)"
  >
    <div v-for="(link, idx) in nginxDirectivesMap?.[props.directive]?.links" :key="idx" class="mb-2">
      <a :href="link">
        {{ link }}
      </a>
    </div>
  </AFormItem>
</template>

<style scoped lang="less">

</style>
