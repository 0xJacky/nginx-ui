<script setup lang="ts">
import CodeEditor from '@/components/CodeEditor'
import {useGettext} from 'vue3-gettext'
import {reactive, ref} from 'vue'

const {$gettext} = useGettext()

const props = defineProps(['locations'])

let location = reactive({
    comments: '',
    path: '',
    content: '',
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
    props.locations?.push(location)
}

function remove(index: number) {
    props.locations?.splice(index, 1)
}
</script>

<template>
    <h2 v-translate>Locations</h2>
    <a-empty v-if="!locations"/>
    <a-card v-for="(v,k) in locations" :key="k"
            :title="$gettext('Location')" size="small">
        <a-form layout="vertical">
            <a-form-item :label="$gettext('Comments')">
                <a-textarea v-model:value="v.comments" :bordered="false"/>
            </a-form-item>
            <a-form-item :label="$gettext('Path')">
                <a-input addon-before="location" v-model:value="v.path"/>
            </a-form-item>
            <a-form-item :label="$gettext('Content')">
                <code-editor v-model:content="v.content" default-height="200px"/>
            </a-form-item>
        </a-form>
    </a-card>

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

    <div>
        <a-button block @click="add">{{ $gettext('Add Location') }}</a-button>
    </div>
</template>

<style lang="less" scoped>
.ant-card {
    margin: 10px 0;
    box-shadow: unset;
}
</style>
