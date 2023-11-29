<script setup lang="ts">
import { useGettext } from 'vue3-gettext'
import Draggable from 'vuedraggable'
import { provide } from 'vue'
import DirectiveAdd from './DirectiveAdd.vue'
import DirectiveEditorItem from '@/views/domain/ngx_conf/directive/DirectiveEditorItem.vue'
import type { NgxDirective } from '@/api/ngx'

defineProps<{
  readonly?: boolean
}>()

const { $gettext } = useGettext()
const current_idx = ref(-1)

const ngx_directives = inject('ngx_directives') as NgxDirective[]

provide('current_idx', current_idx)
</script>

<template>
  <h2>{{ $gettext('Directives') }}</h2>

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
        @click="current_idx = index"
      />
    </template>
  </Draggable>

  <DirectiveAdd
    v-if="!readonly"
    v-auto-animate
    :ngx_directives="ngx_directives"
  />
</template>

<style lang="less" scoped>

</style>
