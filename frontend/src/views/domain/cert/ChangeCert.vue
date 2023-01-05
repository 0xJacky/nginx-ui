<script setup lang="ts">
import {useGettext} from 'vue3-gettext'
import {ref} from 'vue'

const {$gettext} = useGettext()

const props = defineProps(['directivesMap'])

const visible = ref(false)

function open() {
    visible.value = true
}
</script>

<template>
    <div>
        <a-button @click="open">{{ $gettext('Change Certificate') }}</a-button>
        <a-modal
            :title="$gettext('Change Certificate')"
            v-model:visible="visible"
            :mask="false"
        >
            
        </a-modal>
    </div>
</template>

<style lang="less" scoped>

</style>
