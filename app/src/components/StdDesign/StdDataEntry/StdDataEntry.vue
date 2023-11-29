<script setup lang="tsx">
import { Form } from 'ant-design-vue'
import type { Column } from '@/components/StdDesign/types'
import StdFormItem from '@/components/StdDesign/StdDataEntry/StdFormItem.vue'

const props = defineProps<{
  dataList: Column[]
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  dataSource: Record<string, any>
  errors?: Record<string, string>
  layout?: 'horizontal' | 'vertical' | 'inline'
}>()

const emit = defineEmits<{
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  'update:dataSource': [data: Record<string, any>]
}>()

const dataSource = computed({
  get() {
    return props.dataSource
  },
  set(v) {
    emit('update:dataSource', v)
  },
})

const slots = useSlots()

function labelRender(title?: string | (() => string)) {
  if (typeof title === 'function')
    return title()

  return title
}

function extraRender(extra?: string | (() => string)) {
  if (typeof extra === 'function')
    return extra()

  return extra
}

function Render() {
  const template = []

  props.dataList.forEach((v: Column) => {
    let show = true
    if (v.edit?.show && typeof v.edit.show === 'function')
      show = v.edit.show(props.dataSource)

    if (v.edit?.type && show) {
      template.push(<StdFormItem
        dataIndex={v.dataIndex}
      label={labelRender(v.title)}
      extra={extraRender(v.extra)}
      error={props.errors}>
        {v.edit.type(v.edit, dataSource.value, v.dataIndex)}
        </StdFormItem>,
      )
    }
  })

  if (slots.action)
    template.push(<div class={'std-data-entry-action'}>{slots.action()}</div>)

  return <Form layout={props.layout || 'vertical'}>{template}</Form>
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
