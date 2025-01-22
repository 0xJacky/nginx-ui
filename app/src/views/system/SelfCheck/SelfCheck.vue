<script setup lang="ts">
import type { Report } from '@/api/self_check'
import selfCheck from '@/api/self_check'
import { CheckCircleOutlined, CloseCircleOutlined } from '@ant-design/icons-vue'
import tasks from './tasks'

const data = ref<Report[]>()

const loading = ref(false)

function check() {
  loading.value = true
  selfCheck.run().then(r => {
    data.value = r
  }).finally(() => {
    loading.value = false
  })
}

onMounted(() => {
  check()
})

const fixing = reactive({})

function fix(taskName: string) {
  fixing[taskName] = true
  selfCheck.fix(taskName).then(() => {
    check()
  }).finally(() => {
    fixing[taskName] = false
  })
}
</script>

<template>
  <ACard :title="$gettext('Self Check')">
    <template #extra>
      <AButton
        type="link" size="small" :loading @click="check"
      >
        {{ $gettext('Recheck') }}
      </AButton>
    </template>
    <AList :data-source="data">
      <template #renderItem="{ item }">
        <AListItem>
          <template v-if="item.err" #actions>
            <AButton type="link" size="small" :loading="fixing[item.name]" @click="fix(item.name)">
              {{ $gettext('Attempt to fix') }}
            </AButton>
          </template>
          <AListItemMeta>
            <template #title>
              {{ tasks?.[item.name]?.name?.() }}
            </template>
            <template #description>
              {{ tasks?.[item.name]?.description?.() }}
            </template>
            <template #avatar>
              <div class="text-23px">
                <CheckCircleOutlined v-if="!item.err" class="text-green" />
                <CloseCircleOutlined v-else class="text-red" />
              </div>
            </template>
          </AListItemMeta>
        </AListItem>
      </template>
    </AList>
  </ACard>
</template>

<style scoped lang="less">
:deep(.ant-list-item-meta) {
  align-items: center !important;
}
</style>
