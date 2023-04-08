<script setup lang="ts">
import DirectiveAdd from '@/views/domain/ngx_conf/directive/DirectiveAdd'
import {useGettext} from 'vue3-gettext'
import {reactive, ref} from 'vue'
import draggable from 'vuedraggable'
import DirectiveEditorItem from '@/views/domain/ngx_conf/directive/DirectiveEditorItem.vue'

const {$gettext} = useGettext()

const props = defineProps(['ngx_directives', 'readonly'])

const adding = ref(false)

let directive = reactive({})

const current_idx = ref(-1)

function onSave(idx: number) {
    setTimeout(() => {
        current_idx.value = idx + 1
    }, 50)
}
</script>

<template>
    <h2>{{ $gettext('Directives') }}</h2>

    <draggable
        :list="props.ngx_directives"
        item-key="name"
        class="list-group"
        ghost-class="ghost"
        handle=".anticon-holder"
        v-auto-animate
    >
        <template #item="{ element: directive, index }">
            <directive-editor-item @click="current_idx=index"
                                   :directive="directive"
                                   :current_idx="current_idx" :index="index"
                                   :ngx_directives="ngx_directives"
                                   :readonly="readonly"
                                   v-auto-animate
            />
        </template>
    </draggable>

    <directive-add v-if="!readonly" :ngx_directives="ngx_directives" v-auto-animate/>
</template>

<style lang="less" scoped>

</style>
