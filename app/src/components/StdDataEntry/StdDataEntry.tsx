import {defineComponent} from 'vue'
import {Form} from 'ant-design-vue'
import StdFormItem from '@/components/StdDataEntry/StdFormItem.vue'
import './style.less'

export default defineComponent({
  props: ['dataList', 'dataSource', 'error', 'layout'],
  emits: ['update:dataSource'],
  setup(props, {slots}) {
    return () => {
      const template: any = []
      props.dataList.forEach((v: any) => {
        let show = true
        if (v.edit.show) {
          if (typeof v.edit.show === 'boolean') {
            show = v.edit.show
          } else if (typeof v.edit.show === 'function') {
            show = v.edit.show(props.dataSource)
          }
        }
        if (v.edit.type && show) {
          template.push(
            <StdFormItem dataIndex={v.dataIndex} label={v.title()} extra={v.extra} error={props.error}>
              {v.edit.type(v.edit, props.dataSource, v.dataIndex)}
            </StdFormItem>
          )
        }
      })

      if (slots.action) {
        template.push(<div class={'std-data-entry-action'}>{slots.action()}</div>)
      }

      return <Form layout={props.layout || 'vertical'}>{template}</Form>
    }
  }
})
