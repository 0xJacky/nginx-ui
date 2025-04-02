<script setup lang="ts">
import type { TaskReport } from './tasks'
import { CheckCircleOutlined, CloseCircleOutlined, WarningOutlined } from '@ant-design/icons-vue'
import { taskManager } from './tasks'

const data = ref<TaskReport[]>()
const loading = ref(false)

async function check() {
  loading.value = true
  try {
    data.value = await taskManager.runAllChecks()
  }
  finally {
    loading.value = false
  }
}

onMounted(() => {
  check()
})

const fixing = reactive({})

async function fix(taskName: string) {
  fixing[taskName] = true
  try {
    await taskManager.fixTask(taskName)
    check()
  }
  finally {
    fixing[taskName] = false
  }
}
</script>

<template>
  <ACard :title="$gettext('Self Check')">
    <template #extra>
      <AButton
        type="link" size="small" :loading="loading" @click="check"
      >
        {{ $gettext('Recheck') }}
      </AButton>
    </template>
    <AList>
      <AListItem v-for="(item, index) in data" :key="index">
        <template v-if="item.status === 'error'" #actions>
          <AButton type="link" size="small" :loading="fixing[item.name]" @click="fix(item.name)">
            {{ $gettext('Attempt to fix') }}
          </AButton>
        </template>
        <AListItemMeta>
          <template #title>
            {{ taskManager.getTask(item.name)?.name?.() }}
          </template>
          <template #description>
            <div>
              {{ taskManager.getTask(item.name)?.description?.() }}
            </div>
            <div v-if="item.status !== 'success'" class="mt-1">
              <ATag :color="item.status === 'warning' ? 'warning' : 'error'">
                {{ item.message || item.err?.message || $gettext('Unknown issue') }}
              </ATag>
            </div>
          </template>
          <template #avatar>
            <div class="text-23px">
              <CheckCircleOutlined v-if="item.status === 'success'" class="text-green" />
              <WarningOutlined v-else-if="item.status === 'warning'" class="text-yellow" />
              <CloseCircleOutlined v-else class="text-red" />
            </div>
          </template>
        </AListItemMeta>
      </AListItem>
    </AList>
  </ACard>
</template>

<style scoped lang="less">
:deep(.ant-list-item-meta) {
  align-items: center !important;
}

.text-yellow {
  color: #faad14;
}
</style>
