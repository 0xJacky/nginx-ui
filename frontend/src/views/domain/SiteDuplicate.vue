<script setup lang="ts">
import {computed, nextTick, reactive, ref, watch} from 'vue'
import {useGettext} from 'vue3-gettext'
import {Form, message} from 'ant-design-vue'
import gettext from '@/gettext'
import domain from '@/api/domain'

const {$gettext} = useGettext()

const props = defineProps(['visible', 'name'])
const emit = defineEmits(['update:visible', 'duplicated'])

const show = computed({
    get() {
        return props.visible
    },
    set(v) {
        emit('update:visible', v)
    }
})

const modelRef = reactive({name: ''})

const rulesRef = reactive({
    name: [
        {
            required: true,
            message: () => $gettext('Please input name, ' +
                'this will be used as the filename of the new configuration!')
        }
    ]
})

const {validate, validateInfos, clearValidate} = Form.useForm(modelRef, rulesRef)

const loading = ref(false)

function onSubmit() {
    validate().then(async () => {
        loading.value = true

        domain.duplicate(props.name, {name: modelRef.name}).then(() => {
            message.success($gettext('Duplicated successfully'))
            show.value = false
            emit('duplicated')
        }).catch((e: any) => {
            message.error($gettext(e?.message ?? 'Server error'))
        })

        loading.value = false
    })
}

watch(() => props.visible, (v) => {
    if (v) {
        modelRef.name = ''
        nextTick(() => clearValidate())
    }
})

watch(() => gettext.current, () => {
    clearValidate()
})
</script>

<template>
    <a-modal :title="$gettext('Duplicate')" v-model:visible="show" @ok="onSubmit"
             :confirm-loading="loading">
        <a-form layout="vertical">
            <a-form-item :label="$gettext('Name')" v-bind="validateInfos.name">
                <a-input v-model:value="modelRef.name"/>
            </a-form-item>
        </a-form>
    </a-modal>
</template>

<style lang="less" scoped>

</style>
