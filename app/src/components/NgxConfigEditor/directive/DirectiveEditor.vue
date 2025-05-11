<script setup lang="ts">
import type { NgxDirective } from '@/api/ngx'
import Draggable from 'vuedraggable'
import DirectiveAdd from './DirectiveAdd.vue'
import DirectiveEditorItem from './DirectiveEditorItem.vue'
import { useDirectiveStore } from './store'

defineProps<{
  readonly?: boolean
  context?: string
}>()

const directiveStore = useDirectiveStore()
const { curIdx } = storeToRefs(directiveStore)

const ngxDirectives = defineModel<NgxDirective[]>('directives', {
  default: reactive([]),
})

onMounted(() => {
  directiveStore.getNginxDirectivesDocsMap()
})

function addDirective(directive: NgxDirective) {
  ngxDirectives.value.push(directive)
}

function removeDirective(index: number) {
  ngxDirectives.value.splice(index, 1)
}
</script>

<template>
  <div>
    <h3>{{ $gettext('Directives') }}</h3>

    <Draggable
      v-model:list="ngxDirectives"
      item-key="name"
      class="list-group"
      ghost-class="ghost"
      handle=".anticon-holder"
    >
      <template #item="{ index }">
        <DirectiveEditorItem
          v-model:directive="ngxDirectives[index]"
          v-auto-animate
          :index="index"
          :readonly="readonly"
          :context="context"
          @click="curIdx = index"
          @remove="removeDirective(index)"
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
      @save="addDirective"
    />
  </div>
</template>

<style lang="less" scoped>

</style>
