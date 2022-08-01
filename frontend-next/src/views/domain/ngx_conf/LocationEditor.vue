<script setup lang="ts">
import CodeEditor from '@/components/CodeEditor'
import {useGettext} from 'vue3-gettext'
import {reactive} from 'vue'

const {$gettext} = useGettext()

const {locations} = defineProps<{
    locations?: any[]
}>()

let location = reactive({})


function add() {
    adding.value = true
    location = reactive({})
}

function save() {
    this.adding = false
    if (this.locations) {
        this.locations.push(this.location)
    } else {
        this.locations = [this.location]
    }
}

function remove(index) {
    this.locations.splice(index, 1)
}
</script>

<template>
    <h2 v-translate>Locations</h2>
    <a-empty v-if="!locations"/>
    <a-card v-for="(v,k) in locations" :key="k"
            :title="$gettext('Location')" size="small">
        <a-form-item :label="$gettext('Comments')" v-if="v.comments">
            <p style="white-space: pre-wrap;">{{ v.comments }}</p>
        </a-form-item>
        <a-form-item :label="$gettext('Path')">
            <a-input addon-before="location" v-model="v.path"/>
        </a-form-item>
        <a-form-item :label="$gettext('Content')">
            <code-editor v-model:content="v.content" default-height="200px"/>
        </a-form-item>
    </a-card>

    <a-modal :title="$gettext('Add Location')" v-model:visible="adding" @ok="save">
        <a-form-item :label="$gettext('Comments')">
            <a-textarea v-model="location.comments"></a-textarea>
        </a-form-item>
        <a-form-item :label="$gettext('Path')">
            <a-input addon-before="location" v-model="location.path"/>
        </a-form-item>
        <a-form-item :label="$gettext('Content')">
            <vue-itextarea v-model:content="location.content" default-height="200px"/>
        </a-form-item>
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
