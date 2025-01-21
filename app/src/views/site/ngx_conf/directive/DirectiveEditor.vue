<script setup lang="ts">
import type { DirectiveMap, NgxDirective } from '@/api/ngx'
import type { ComputedRef } from 'vue'
import ngx from '@/api/ngx'
import DirectiveEditorItem from '@/views/site/ngx_conf/directive/DirectiveEditorItem.vue'
import Draggable from 'vuedraggable'
import DirectiveAdd from './DirectiveAdd.vue'

defineProps<{
  readonly?: boolean
  context?: string
}>()

const current_idx = ref(-1)

const ngx_directives = inject('ngx_directives') as ComputedRef<NgxDirective[]>

provide('current_idx', current_idx)

const nginxDirectivesMap = shallowRef<DirectiveMap>()

onMounted(async () => {
  nginxDirectivesMap.value = await ngx.get_directives()
})
</script>

<template>
  <h3>{{ $gettext('Directives') }}</h3>

  <Draggable
    :list="ngx_directives"
    item-key="name"
    class="list-group"
    ghost-class="ghost"
    handle=".anticon-holder"
  >
    <template #item="{ index }">
      <DirectiveEditorItem
        v-auto-animate
        :index="index"
        :readonly="readonly"
        :context="context"
        :nginx-directives-map
        @click="current_idx = index"
      >
        <template
          v-if="$slots.directiveSuffix"
          #suffix="{ directive }"
        >
          <slot
            name="directiveSuffix"
            :directive="directive"
          />
        </template>
      </DirectiveEditorItem>
    </template>
  </Draggable>

  <DirectiveAdd
    v-if="!readonly"
    v-auto-animate
    :nginx-directives-map
  />
</template>

<style lang="less" scoped>

</style>
