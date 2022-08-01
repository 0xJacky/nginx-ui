import {defineComponent} from 'vue'
import {Form, FormItem} from 'ant-design-vue'

export default defineComponent({
    props: ['dataList', 'dataSource', 'error', 'layout'],
    emits: ['update:dataSource'],
    setup(props, {slots}) {
        return () => {
            const template: any = []
            props.dataList.forEach((v: any) => {
                if (v.edit.type) {
                    template.push(
                        <FormItem label={v.title()}>
                            {v.edit.type(v.edit, props.dataSource, v.dataIndex)}
                        </FormItem>
                    )
                }
            })

            if (slots.action) {
                template.push(<div>{slots.action()}</div>)
            }

            return <Form layout={props.layout || 'vertical'}>{template}</Form>
        }
    }
})
