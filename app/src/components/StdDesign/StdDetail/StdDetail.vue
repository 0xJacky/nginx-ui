<script setup lang="ts" generic="T extends ModelBase">
import type { ModelBase } from '@/api/curd'
import type Curd from '@/api/curd'
import type { Column, StdDesignEdit } from '@/components/StdDesign/types'
import type { ButtonProps, FormInstance } from 'ant-design-vue'
import type { DataIndex } from 'ant-design-vue/es/vc-table/interface'
import { labelRender } from '@/components/StdDesign/StdDataEntry'
import { message } from 'ant-design-vue'

import _, { get } from 'lodash'

const props = defineProps<{
  title?: string
  dataSource?: T
  api: Curd<T>
  columns: Column[]
  actionButtonProps?: ButtonProps
  useOutsideData?: boolean
}>()

const detail = ref(props.dataSource) as Ref<T | undefined>
const editModel = ref({}) as Ref<T | undefined>
const editStatus = ref(false)
const loading = ref(false)

const formRef = ref<FormInstance>()

watch(() => props.dataSource, val => detail.value = val)

async function save() {
  try {
    await formRef.value?.validate()
    loading.value = true
    props.api.save(editModel.value?.id, editModel.value).then(res => {
      detail.value = res
      editStatus.value = false
    }).catch(() => {
      message.error('Save failed')
    }).finally(() => loading.value = false)
  }
  catch {
    message.error('Validation failed')
  }
}

function FormController(p: { editConfig: StdDesignEdit, dataIndex?: DataIndex }) {
  return p?.editConfig?.type?.(p.editConfig, editModel.value, p.dataIndex)
}

function CustomRender(p: { column?: Column, text: unknown, record?: T }) {
  const { column, text, record } = p
  return column?.customRender?.({ text, record }) ?? text ?? '/'
}

const route = useRoute()

onMounted(() => {
  if (props?.useOutsideData) {
    editModel.value = _.cloneDeep(props.dataSource)
    return
  }

  props.api.get(route.params.id).then(res => {
    detail.value = res
  })
})

function clickEdit() {
  editModel.value = _.cloneDeep(detail.value)
  editStatus.value = true
}
</script>

<template>
  <AForm
    ref="formRef"
    :model="editModel"
  >
    <ADescriptions
      bordered
      :title="props.title ?? $gettext('Info')"
      :column="2"
    >
      <template #extra>
        <ASpace v-if="editStatus">
          <AButton
            type="primary"
            :disabled="loading"
            :loading="loading"
            v-bind="props.actionButtonProps"
            @click="save"
          >
            {{ $gettext('Save') }}
          </AButton>
          <AButton
            :disabled="loading"
            :loading="loading"
            v-bind="props.actionButtonProps"
            @click="editStatus = false"
          >
            {{ $gettext('Cancel') }}
          </AButton>
        </ASpace>
        <div v-else>
          <AButton
            type="primary"
            v-bind="props.actionButtonProps"
            @click="clickEdit"
          >
            {{ $gettext('Edit') }}
          </AButton>
          <slot name="extra" />
        </div>
      </template>
      <ADescriptionsItem
        v-for="c in props.columns.filter(c => c.dataIndex !== 'action')"
        :key="c.dataIndex?.toString()"
        :label="$gettext(labelRender(c.title) ?? '')"
      >
        <AFormItem
          v-if="editStatus && c.edit"
          class="mb-0"
          :name="c.dataIndex?.toString()"
          :required="c?.edit?.config?.required"
        >
          <FormController
            :edit-config="c.edit"
            :data-index="c.dataIndex"
          />
        </AFormItem>
        <span v-else>
          <CustomRender
            :column="c"
            :text="get(detail, c.dataIndex as any)"
            :record="detail"
          />
        </span>
      </ADescriptionsItem>
    </ADescriptions>
  </AForm>
</template>
