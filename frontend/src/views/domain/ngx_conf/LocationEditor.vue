<script setup lang="ts">
import CodeEditor from '@/components/CodeEditor'
import {useGettext} from 'vue3-gettext'
import {reactive, ref} from 'vue'
import {DeleteOutlined, HolderOutlined} from '@ant-design/icons-vue'
import draggable from 'vuedraggable'

const {$gettext} = useGettext()

const props = defineProps(['locations', 'readonly'])

let location = reactive({
    comments: '',
    path: '',
    content: ''
})

const adding = ref(false)

function add() {
    adding.value = true
    location.comments = ''
    location.path = ''
    location.content = ''
}

function save() {
    adding.value = false
    props.locations?.push({
        ...location
    })
}

function remove(index: number) {
    props.locations?.splice(index, 1)
}
</script>

<template>
    <h2 v-translate>Locations</h2>
    <a-empty v-if="!locations"/>
    <draggable
        :list="locations"
        item-key="name"
        class="list-group"
        ghost-class="ghost"
        handle=".ant-card-head"
    >
        <template #item="{ element: v, index }">
            <a-card :key="index" size="small">
                <template #title>
                    <HolderOutlined/>
                    {{ $gettext('Location') }}
                </template>
                <template #extra v-if="!readonly">
                    <a-popconfirm @confirm="remove(index)"
                                  :title="$gettext('Are you sure you want to remove this location?')"
                                  :ok-text="$gettext('Yes')"
                                  :cancel-text="$gettext('No')">
                        <a-button type="text">
                            <template #icon>
                                <DeleteOutlined style="font-size: 14px;"/>
                            </template>
                        </a-button>
                    </a-popconfirm>
                </template>

                <a-form layout="vertical">
                    <a-form-item :label="$gettext('Comments')">
                        <a-textarea v-model:value="v.comments" :bordered="false"/>
                    </a-form-item>
                    <a-form-item :label="$gettext('Path')">
                        <a-input addon-before="location" v-model:value="v.path"/>
                    </a-form-item>
                    <a-form-item :label="$gettext('Content')">
                        <code-editor v-model:content="v.content" default-height="200px" style="width: 100%;"/>
                    </a-form-item>
                </a-form>
            </a-card>
        </template>
    </draggable>

    <a-modal :title="$gettext('Add Location')" v-model:visible="adding" @ok="save">
        <a-form layout="vertical">
            <a-form-item :label="$gettext('Comments')">
                <a-textarea v-model:value="location.comments"/>
            </a-form-item>
            <a-form-item :label="$gettext('Path')">
                <a-input addon-before="location" v-model:value="location.path"/>
            </a-form-item>
            <a-form-item :label="$gettext('Content')">
                <code-editor v-model:content="location.content" default-height="200px"/>
            </a-form-item>
        </a-form>
    </a-modal>

    <div v-if="!readonly">
        <a-button block @click="add">{{ $gettext('Add Location') }}</a-button>
    </div>
</template>

<style lang="less" scoped>
.ant-card {
    margin: 10px 0;
    box-shadow: unset;
}
</style>
