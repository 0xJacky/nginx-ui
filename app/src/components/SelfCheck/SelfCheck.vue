<script setup lang="ts">
import { CheckCircleOutlined, CloseCircleOutlined, WarningOutlined } from '@ant-design/icons-vue'
import { useSelfCheckStore } from './store'

const store = useSelfCheckStore()

const { data, loading, fixing } = storeToRefs(store)

onMounted(() => {
  store.check()
})
</script>

<template>
  <ACard :title="$gettext('Self Check')">
    <template #extra>
      <AButton
        type="link"
        size="small"
        :loading="loading"
        @click="store.check"
      >
        {{ $gettext('Recheck') }}
      </AButton>
    </template>
    <AList>
      <AListItem v-for="(item, index) in data" :key="index">
        <template v-if="item.status === 'error' && item.fixable" #actions>
          <AButton type="link" size="small" :loading="fixing[item.key]" @click="store.fix(item.key)">
            {{ $gettext('Attempt to fix') }}
          </AButton>
        </template>
        <AListItemMeta>
          <template #title>
            {{ item.name?.() }}
          </template>
          <template #description>
            <div>
              {{ item.description?.() }}
            </div>
            <div v-if="item.status !== 'success'" class="mt-1">
              <ATag :color="item.status === 'warning' ? 'warning' : 'error'">
                {{ item.err?.message || $gettext('Unknown issue') }}
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
