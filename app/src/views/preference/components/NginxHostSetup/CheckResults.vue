<script setup lang="ts">
import type { CheckRow } from './checks'
import { CheckCircleOutlined, CloseCircleOutlined, ExclamationCircleOutlined } from '@ant-design/icons-vue'
import { computed } from 'vue'
import { tagColor, tagText } from './checks'

const props = defineProps<{ rows: CheckRow[] }>()

const failed = computed(() => props.rows.filter(row => row.level === 'error'))
const warnings = computed(() => props.rows.filter(row => row.level === 'warning'))
const allPassed = computed(() => props.rows.length > 0 && failed.value.length === 0)
</script>

<template>
  <div v-if="rows.length" class="space-y-4">
    <AAlert
      v-if="failed.length"
      type="error"
      show-icon
      :message="$gettext('Some checks failed. Fix them before continuing.')"
    >
      <template #description>
        <ul class="m-0 pl-4">
          <li v-for="row in failed" :key="row.key">
            {{ row.label }}
          </li>
        </ul>
      </template>
    </AAlert>
    <AAlert
      v-else-if="allPassed && warnings.length"
      type="warning"
      show-icon
      :message="$gettext('Blocking checks passed. Review the warnings below.')"
    />
    <AAlert
      v-else-if="allPassed"
      type="success"
      show-icon
      :message="$gettext('All checks passed.')"
    />

    <AList :data-source="rows">
      <template #renderItem="{ item }">
        <AListItem>
          <ASpace direction="vertical" size="small" class="w-full">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <ASpace :size="8">
                <CheckCircleOutlined v-if="item.level === 'success'" :style="{ color: '#52c41a' }" />
                <ExclamationCircleOutlined v-else-if="item.level === 'warning'" :style="{ color: '#faad14' }" />
                <CloseCircleOutlined v-else :style="{ color: '#ff4d4f' }" />
                <strong>{{ item.label }}</strong>
              </ASpace>
              <ATag :color="tagColor(item.level)" :bordered="false">
                {{ tagText(item.level) }}
              </ATag>
            </div>
            <ATypographyText type="secondary" class="break-words text-sm">
              {{ item.outcome.detail }}
            </ATypographyText>
            <ASpace v-if="item.outcome.remediation" direction="vertical" size="small" class="w-full">
              <ATypographyText type="secondary" class="text-xs">
                {{ $gettext('Suggested fix') }}
              </ATypographyText>
              <ATypographyParagraph
                class="mb-0 break-all"
                code
                :copyable="{ text: item.outcome.remediation, tooltip: false }"
              >
                {{ item.outcome.remediation }}
              </ATypographyParagraph>
            </ASpace>
          </ASpace>
        </AListItem>
      </template>
    </AList>
  </div>
</template>
