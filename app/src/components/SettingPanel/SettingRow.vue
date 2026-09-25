<script setup lang="ts">
import { SETTING_ROW_CONTEXT } from './context'

const props = defineProps<{
  title: string
  description?: string
  // Dot path inside the settings object, used for search and change tracking.
  path?: string
  // Read-only value shown when no control is passed in the default slot.
  value?: string | number | null
  // Puts the control under the text instead of at the right.
  stacked?: boolean
  requiresRestart?: boolean
  // Name of the app.ini section that owns this value. Marks the row as read-only.
  configFile?: string
  error?: string
}>()

const context = inject(SETTING_ROW_CONTEXT, undefined)
const root = useTemplateRef<HTMLElement>('root')

const isDirty = computed(() => !!props.path && !!context?.changedPaths.value.has(props.path))
const isHighlighted = computed(() => !!props.path && context?.highlightedPath.value === props.path)

const resolvedDescription = computed(() => {
  if (props.description)
    return props.description
  if (props.configFile)
    return $gettext('Change it in the %{section} section of app.ini.', { section: props.configFile })
  return ''
})

const hasValue = computed(() => props.value !== undefined && props.value !== null && props.value !== '')

watch(isHighlighted, async value => {
  if (!value)
    return
  await nextTick()
  root.value?.scrollIntoView({ block: 'center', behavior: 'smooth' })
}, { immediate: true })
</script>

<template>
  <div
    ref="root"
    class="setting-row"
    :class="{
      'is-stacked': stacked,
      'is-highlighted': isHighlighted,
      'is-dirty': isDirty,
      'has-error': !!error,
    }"
    :data-setting-path="path"
  >
    <div class="setting-text">
      <div class="setting-title">
        <span class="setting-title-text">
          <slot name="title">
            {{ title }}
          </slot>
        </span>
        <span
          v-if="isDirty"
          class="setting-dirty-dot"
          :title="$gettext('Unsaved')"
        />
        <ATag
          v-if="requiresRestart"
          color="warning"
          class="setting-tag"
        >
          {{ $gettext('Restart required') }}
        </ATag>
        <ATag
          v-if="configFile"
          class="setting-tag setting-tag-file"
        >
          {{ $gettext('Config file') }}
        </ATag>
        <slot name="tags" />
      </div>
      <div
        v-if="resolvedDescription || $slots.description"
        class="setting-desc"
      >
        <slot name="description">
          {{ resolvedDescription }}
        </slot>
      </div>
      <div
        v-if="error"
        class="setting-error"
      >
        {{ error }}
      </div>
    </div>
    <div class="setting-control">
      <slot>
        <span
          class="setting-value"
          :class="{ 'is-empty': !hasValue }"
        >
          {{ hasValue ? value : $gettext('Not set') }}
        </span>
      </slot>
    </div>
  </div>
</template>

<style lang="less" scoped>
.setting-row {
  position: relative;
  z-index: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24px;
  padding: 14px 0;
  border-top: 1px solid var(--ant-color-split);

  // Sits behind the row content so the highlight never covers the text.
  &::before {
    content: '';
    position: absolute;
    z-index: -1;
    inset: 4px -12px;
    border-radius: 8px;
    pointer-events: none;
  }

  &.is-highlighted::before {
    animation: setting-row-flash 2.4s ease-out;
  }

  &.is-stacked {
    flex-direction: column;
    align-items: stretch;
    gap: 10px;
  }
}

@keyframes setting-row-flash {
  0% {
    background: var(--ant-color-primary-bg);
  }
  70% {
    background: var(--ant-color-primary-bg);
  }
  100% {
    background: transparent;
  }
}

.setting-text {
  flex: 1 1 auto;
  min-width: 0;
}

.setting-title {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  font-weight: 500;
}

.setting-dirty-dot {
  width: 6px;
  height: 6px;
  border-radius: 3px;
  background: var(--ant-color-warning);
}

.setting-tag {
  margin-inline-end: 0;
  font-weight: 400;
}

.setting-tag-file {
  color: var(--ant-color-text-tertiary);
}

.setting-desc {
  margin-top: 2px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--ant-color-text-secondary);
  overflow-wrap: anywhere;
}

.setting-error {
  margin-top: 2px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--ant-color-error);
}

.setting-control {
  display: flex;
  flex: none;
  justify-content: flex-end;
  align-items: center;
  max-width: 50%;
  min-width: 0;

  .is-stacked & {
    display: block;
    max-width: 100%;
  }
}

.setting-value {
  color: var(--ant-color-text-secondary);
  text-align: right;
  overflow-wrap: anywhere;

  &.is-empty {
    color: var(--ant-color-text-quaternary);
  }
}

@media (max-width: 600px) {
  .setting-row {
    flex-direction: column;
    align-items: stretch;
    gap: 10px;
  }

  .setting-control {
    justify-content: flex-start;
    max-width: 100%;
  }

  .setting-value {
    text-align: left;
  }
}
</style>
