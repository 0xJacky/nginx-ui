<script setup lang="ts" generic="T=any">
import type Curd from '@/api/curd'
import type { BulkActionOptions, BulkActions } from '@/components/StdDesign/types'
import { message } from 'ant-design-vue'

const props = defineProps<{
  api: Curd<T>
  actions: BulkActions
  selectedRowKeys: Array<number | string>
  inTrash?: boolean
}>()

const emit = defineEmits(['onSuccess'])

const computedActions = computed(() => {
  if (!props.inTrash) {
    const result = { ...props.actions }

    if (result.delete) {
      result.delete = {
        text: () => $gettext('Delete'),
        action: ids => {
          return props.api.batch_destroy(ids)
        },
      }
    }
    if (result.recover)
      delete result.recover
    return result
  }
  else {
    const result = {} as { [key: string]: BulkActionOptions }
    if (props.actions.delete) {
      result.delete = {
        text: () => $gettext('Delete Permanently'),
        action: ids => {
          return props.api.batch_destroy(ids, { permanent: true })
        },
      }
    }
    if (props.actions.recover) {
      result.recover = {
        text: () => $gettext('Recover'),
        action: ids => {
          return props.api.batch_recover(ids)
        },
      }
    }
    return result
  }
}) as ComputedRef<Record<string, BulkActionOptions>>

const actionValue = ref('')

watch(() => props.inTrash, () => {
  actionValue.value = ''
})

function onClickApply() {
  return new Promise(resolve => {
    if (actionValue.value === '')
      return resolve(false)

    // call action
    return resolve(
      computedActions.value[actionValue.value]?.action(props.selectedRowKeys).then(async () => {
        message.success($gettext('Apply bulk action successfully'))
        emit('onSuccess')
      }),
    )
  })
}
</script>

<template>
  <AFormItem>
    <ASpace>
      <ASelect
        v-model:value="actionValue"
        style="min-width: 150px"
      >
        <ASelectOption value="">
          {{ $gettext('Batch Actions') }}
        </ASelectOption>
        <ASelectOption
          v-for="(action, key) in computedActions"
          :key
          :value="key"
        >
          {{ action.text() }}
        </ASelectOption>
      </ASelect>
      <APopconfirm
        :cancel-text="$gettext('No')"
        :ok-text="$gettext('OK')"
        :title="$gettext('Are you sure you want to apply to all selected?')"
        @confirm="onClickApply"
      >
        <AButton
          danger
          :disabled="!actionValue || !selectedRowKeys?.length"
        >
          {{ $gettext('Apply') }}
        </AButton>
      </APopconfirm>
    </ASpace>
  </AFormItem>
</template>
