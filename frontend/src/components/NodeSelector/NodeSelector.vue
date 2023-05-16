<script setup lang="ts">
import {computed, ref} from 'vue'
import environment from '@/api/environment'
import {useGettext} from 'vue3-gettext'

const {$gettext} = useGettext()

const props = defineProps(['target', 'map', 'hidden_local'])
const emit = defineEmits(['update:target'])

const data = ref([])
const data_map = ref({})

environment.get_list().then(r => {
    data.value = r.data
    r.data.forEach(node => {
        data_map[node.id] = node
    })
})

const value = computed({
    get() {
        return props.target
    },
    set(v) {
        if (typeof props.map === 'object') {
            v.forEach(id => {
                if (id !== 0) props.map[id] = data_map[id].name
            })
        }
        emit('update:target', v)
    }
})
</script>

<template>
    <a-checkbox-group v-model:value="value" style="width: 100%">
        <a-row :gutter="[16,16]">
            <a-col :span="8" v-if="!hidden_local">
                <a-checkbox :value="0">{{ $gettext('Local') }}</a-checkbox>
                <a-tag color="blue">{{ $gettext('Online') }}</a-tag>
            </a-col>
            <a-col :span="8" v-for="node in data">
                <a-checkbox :value="node.id">{{ node.name }}</a-checkbox>
                <a-tag color="blue" v-if="node.status">{{ $gettext('Online') }}</a-tag>
                <a-tag color="error" v-else>{{ $gettext('Offline') }}</a-tag>
            </a-col>
        </a-row>
    </a-checkbox-group>
</template>

<style scoped lang="less">

</style>
