<script setup lang="tsx">
import type { Column, JSXElements, StdDesignEdit } from '@/components/StdDesign/types'
import type { FormInstance } from 'ant-design-vue'
import type { Ref } from 'vue'
import { labelRender } from '@/components/StdDesign/StdDataEntry'
import StdFormItem from '@/components/StdDesign/StdDataEntry/StdFormItem.vue'
import { Form } from 'ant-design-vue'

const props = defineProps<{
  dataList: Column[]
  errors?: Record<string, string>
  type?: 'search' | 'edit'
  layout?: 'horizontal' | 'vertical' | 'inline'
}>()

// eslint-disable-next-line ts/no-explicit-any
const dataSource = defineModel<Record<string, any>>('dataSource')

const slots = useSlots()

function extraRender(extra?: string | (() => string)) {
  if (typeof extra === 'function')
    return extra()

  return extra
}

const formRef = ref<FormInstance>()

defineExpose({
  formRef,
})

function Render() {
  const template: JSXElements = []
  const isCreate = inject<Ref<string>>('editMode', ref(''))?.value === 'create'

  props.dataList.forEach((v: Column) => {
    const dataIndex = (v.edit?.actualDataIndex ?? v.dataIndex) as string

    dataSource.value![dataIndex] = dataSource.value![dataIndex]
    if (props.type === 'search') {
      if (v.search) {
        const type = (v.search as StdDesignEdit)?.type || v.edit?.type

        template.push(
          <StdFormItem
            label={labelRender(v.title)}
            extra={extraRender(v.extra)}
            error={props.errors}
          >
            {type?.(v.edit!, dataSource.value, v.dataIndex)}
          </StdFormItem>,
        )
      }

      return
    }

    // console.log(isCreate && v.hiddenInCreate, !isCreate && v.hiddenInModify)
    if ((isCreate && v.hiddenInCreate) || (!isCreate && v.hiddenInModify))
      return

    let show = true
    if (v.edit?.show && typeof v.edit.show === 'function')
      show = v.edit.show(dataSource.value)

    if (v.edit?.type && show) {
      template.push(
        <StdFormItem
          key={dataIndex}
          dataIndex={dataIndex}
          label={labelRender(v.title)}
          extra={extraRender(v.extra)}
          error={props.errors}
          required={v.edit?.config?.required}
          hint={v.edit?.hint}
          noValidate={v.edit?.config?.noValidate}
        >
          {v.edit.type(v.edit, dataSource.value, dataIndex)}
        </StdFormItem>,
      )
    }
  })

  if (slots.action)
    template.push(<div class="std-data-entry-action">{slots.action()}</div>)

  return (
    <Form
      class="my-10px!"
      ref={formRef}
      model={dataSource.value}
      layout={props.layout || 'vertical'}
    >
      {template}
    </Form>
  )
}
</script>

<template>
  <Render />
</template>

<style scoped lang="less">
.std-data-entry-action {
  @media (max-width: 375px) {
    display: block;
    width: 100%;
    margin: 10px 0;
  }
}
</style>
