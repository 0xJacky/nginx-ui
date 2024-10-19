<script setup lang="ts">
import { type ComputedRef, reactive, ref } from 'vue'
import { DeleteOutlined } from '@ant-design/icons-vue'
import CodeEditor from '@/components/CodeEditor'
import type { NgxDirective } from '@/api/ngx'

const props = defineProps<{
  idx?: number
}>()

const emit = defineEmits(['save'])

const ngx_directives = inject('ngx_directives') as ComputedRef<NgxDirective[]>
const directive = reactive({ directive: '', params: '' })
const adding = ref(false)
const mode = ref('default')

function add() {
  adding.value = true
  directive.directive = ''
  directive.params = ''
}

function save() {
  adding.value = false
  if (mode.value === 'multi-line')
    directive.directive = ''

  if (props.idx)
    ngx_directives.value.splice(props.idx + 1, 0, { directive: directive.directive, params: directive.params })
  else
    ngx_directives.value.push({ directive: directive.directive, params: directive.params })

  emit('save', props.idx)
}
</script>

<template>
  <div>
    <div
      v-if="adding"
      class="add-directive-temp"
    >
      <AFormItem>
        <ASelect
          v-model:value="mode"
          default-value="default"
          style="width: 180px"
        >
          <ASelectOption value="default">
            {{ $gettext('Single Directive') }}
          </ASelectOption>
          <ASelectOption value="multi-line">
            {{ $gettext('Multi-line Directive') }}
          </ASelectOption>
        </ASelect>
      </AFormItem>
      <AFormItem>
        <div class="input-wrapper">
          <CodeEditor
            v-if="mode === 'multi-line'"
            v-model:content="directive.params"
            default-height="100px"
            style="width: 100%;"
          />
          <AInputGroup
            v-else
            compact
          >
            <AInput
              v-model:value="directive.directive"
              style="width: 30%"
              :placeholder="$gettext('Directive')"
            />
            <AInput
              v-model:value="directive.params"
              style="width: 70%"
              :placeholder="$gettext('Params')"
            />
          </AInputGroup>

          <AButton @click="adding = false">
            <template #icon>
              <DeleteOutlined style="font-size: 14px;" />
            </template>
          </AButton>
        </div>
      </AFormItem>
    </div>
    <AButton
      v-if="!adding"
      block
      @click="add"
    >
      {{ $gettext('Add Directive Below') }}
    </AButton>
    <AButton
      v-else
      type="primary"
      block
      :disabled="(mode === 'default' && (!directive.directive || !directive.params))
        || (!directive.params && mode === 'multi-line')"
      @click="save"
    >
      {{ $gettext('Save Directive') }}
    </AButton>
  </div>
</template>

<style lang="less" scoped>
.input-wrapper {
  display: flex;
  gap: 10px;
  align-items: center;
}
</style>
