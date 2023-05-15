<script setup lang="ts">
import {computed, ref} from 'vue'
import environment from '@/api/environment'
import {useGettext} from 'vue3-gettext'

const {$gettext} = useGettext()

const props = defineProps(['target', 'map'])
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
        <a-row>
            <a-col :span="8">
                <a-checkbox :value="0">{{ $gettext('Local') }}</a-checkbox>
                <a-badge color="green"/>
            </a-col>
            <a-col :span="8" v-for="node in data">
                <a-checkbox :value="node.id">{{ node.name }}</a-checkbox>
                <a-badge color="green" v-if="node.status"/>
                <a-badge color="red" v-else/>
            </a-col>
        </a-row>
    </a-checkbox-group>
</template>

<style scoped lang="less">

</style>
