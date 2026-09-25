<script setup lang="ts">
import FooterToolBar from '@/components/FooterToolbar'
import gettext from '@/gettext'

const props = defineProps<{
  labels: string[]
  saving?: boolean
}>()

const emit = defineEmits<{
  save: []
  discard: []
}>()

const count = computed(() => props.labels.length)

const countText = computed(() => $ngettext(
  '%{count} unsaved change',
  '%{count} unsaved changes',
  count.value,
  { count: String(count.value) },
))

// Chinese and Japanese separate list items with "、".
const summary = computed(() => props.labels.join(/^(?:zh|ja)/.test(gettext.current) ? '、' : ', '))
</script>

<template>
  <FooterToolBar>
    <template #extra>
      <div class="preference-savebar-info">
        <ATag
          color="warning"
          class="preference-savebar-badge"
        >
          {{ countText }}
        </ATag>
        <span
          class="preference-savebar-summary"
          :title="summary"
        >
          {{ summary }}
        </span>
      </div>
    </template>
    <ASpace>
      <APopconfirm
        :title="$gettext('Discard all unsaved changes?')"
        :ok-text="$gettext('Discard')"
        :cancel-text="$gettext('Keep editing')"
        placement="topRight"
        @confirm="emit('discard')"
      >
        <AButton :disabled="saving">
          {{ $gettext('Discard') }}
        </AButton>
      </APopconfirm>
      <AButton
        type="primary"
        :loading="saving"
        @click="emit('save')"
      >
        {{ $gettext('Save') }}
      </AButton>
    </ASpace>
  </FooterToolBar>
</template>

<style lang="less" scoped>
.preference-savebar-info {
  display: flex;
  align-items: center;
  gap: 12px;
  height: 56px;
  max-width: 60vw;
  line-height: normal;
}

.preference-savebar-badge {
  margin-inline-end: 0;
  border-radius: 10px;
}

.preference-savebar-summary {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--ant-color-text-secondary);
}

@media (max-width: 600px) {
  .preference-savebar-summary {
    display: none;
  }
}
</style>
