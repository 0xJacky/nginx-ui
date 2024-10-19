<script setup lang="ts">
import Draggable from 'vuedraggable'
import type { ComputedRef } from 'vue'
import DirectiveAdd from './DirectiveAdd.vue'
import DirectiveEditorItem from '@/views/site/ngx_conf/directive/DirectiveEditorItem.vue'
import type { NgxDirective } from '@/api/ngx'

defineProps<{
  readonly?: boolean
  context?: string
}>()

const current_idx = ref(-1)

const ngx_directives = inject('ngx_directives') as ComputedRef<NgxDirective[]>

provide('current_idx', current_idx)
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
  />
</template>

<style lang="less" scoped>

</style>
