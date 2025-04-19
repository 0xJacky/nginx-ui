<script setup lang="ts">
import CodeEditor from '@/components/CodeEditor'
import { DeleteOutlined } from '@ant-design/icons-vue'
import { MultiLineDirective, SingleLineDirective } from '.'
import { useDirectiveStore } from './store'

const emit = defineEmits(['save'])

const directiveStore = useDirectiveStore()
const { nginxDirectivesDocsMap, nginxDirectivesOptions } = storeToRefs(directiveStore)

const directive = reactive({ directive: '', params: '' })
const adding = ref(false)
const mode = ref(SingleLineDirective)

function add() {
  adding.value = true
  directive.directive = ''
  directive.params = ''
}

function save() {
  adding.value = false
  if (mode.value === MultiLineDirective)
    directive.directive = ''

  emit('save', directive)
}

function filterOption(inputValue: string, option: { label: string }) {
  return option.label.toLowerCase().includes(inputValue.toLowerCase())
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
          :default-value="SingleLineDirective"
          style="width: 180px"
        >
          <ASelectOption :value="SingleLineDirective">
            {{ $gettext('Single Directive') }}
          </ASelectOption>
          <ASelectOption :value="MultiLineDirective">
            {{ $gettext('Multi-line Directive') }}
          </ASelectOption>
        </ASelect>
      </AFormItem>
      <AFormItem>
        <div class="input-wrapper">
          <CodeEditor
            v-if="mode === MultiLineDirective"
            v-model:content="directive.params"
            default-height="100px"
            style="width: 100%;"
          />
          <AInputGroup
            v-else
            compact
          >
            <AAutoComplete
              v-model:value="directive.directive"
              :options="nginxDirectivesOptions"
              style="width: 30%"
              :filter-option="filterOption"
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
        <div v-if="nginxDirectivesDocsMap?.[directive.directive]" class="mt-2">
          <div>{{ $ngettext('Document', 'Documents', nginxDirectivesDocsMap[directive.directive].links.length) }}</div>
          <div v-for="(link, index) in nginxDirectivesDocsMap?.[directive.directive].links" :key="index" class="overflow-auto">
            <a :href="link">
              {{ link }}
            </a>
          </div>
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
