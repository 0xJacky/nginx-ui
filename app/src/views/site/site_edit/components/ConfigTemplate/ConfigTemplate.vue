<script setup lang="ts">
import type { Template } from '@/api/template'
import { SearchOutlined } from '@ant-design/icons-vue'
import { storeToRefs } from 'pinia'
import template from '@/api/template'
import CodeEditor from '@/components/CodeEditor'
import { DirectiveEditor, LocationEditor, useNgxConfigStore } from '@/components/NgxConfigEditor'
import { useSettingsStore } from '@/pinia'
import { useConfigTemplateStore } from './store'
import TemplateForm from './TemplateForm.vue'

const { language } = storeToRefs(useSettingsStore())

const ngxConfigStore = useNgxConfigStore()
const { ngxConfig, curServer } = storeToRefs(ngxConfigStore)

const configTemplateStore = useConfigTemplateStore()
const { data } = storeToRefs(configTemplateStore)

const blocks = ref<Template[]>([])
const visible = ref(false)
const name = ref('')
const filterText = ref('')

function getBlockList() {
  template.get_block_list().then(r => {
    blocks.value = r.data
  })
}

getBlockList()

function view(n: string) {
  visible.value = true
  name.value = n
  template.get_block(n).then(r => {
    data.value = r
  })
}

const transDescription = computed(() => {
  return (item: { description: { [key: string]: string } }) =>
    item.description?.[language.value] ?? item.description?.en ?? ''
})

const filteredBlocks = computed(() => {
  if (!filterText.value)
    return blocks.value

  const searchText = filterText.value.toLowerCase()
  return blocks.value.filter(item =>
    item.name?.toLowerCase().includes(searchText)
    || item.author?.toLowerCase().includes(searchText)
    || transDescription.value(item).toLowerCase().includes(searchText),
  )
})

async function add() {
  if (data.value?.custom)
    ngxConfig.value.custom += `\n${data.value.custom}`

  ngxConfig.value.custom = ngxConfig.value.custom?.trim()

  if (data.value?.locations)
    curServer.value?.locations?.push(...data.value.locations)

  if (data.value?.directives)
    curServer.value?.directives?.push(...data.value.directives)

  visible.value = false
}
</script>

<template>
  <div>
    <div class="mb-4">
      <AInput
        v-model:value="filterText"
        :placeholder="$gettext('Search templates')"
        allow-clear
      >
        <template #prefix>
          <SearchOutlined />
        </template>
      </AInput>
    </div>
    <div class="config-list-wrapper">
      <AList :data-source="filteredBlocks">
        <template #renderItem="{ item }">
          <AListItem>
            <AListItemMeta
              :title="item.name"
            >
              <template #description>
                <p class="mt-4">
                  {{ $gettext('Author') }}: {{ item.author }}
                </p>
                <p class="mb-0">
                  {{ $gettext('Description') }}: {{ transDescription(item) }}
                </p>
              </template>
            </AListItemMeta>
            <template #extra>
              <AButton
                type="link"
                @click="view(item.filename)"
              >
                {{ $gettext('View') }}
              </AButton>
            </template>
          </AListItem>
        </template>
      </AList>
    </div>
    <AModal
      v-model:open="visible"
      :title="data.name"
      :mask="false"
      :ok-text="$gettext('Add')"
      @ok="add"
    >
      <p>{{ $gettext('Author') }}: {{ data.author }}</p>
      <p>{{ $gettext('Description') }}: {{ transDescription(data) }}</p>
      <TemplateForm v-model="data.variables" />
      <div
        v-if="data.custom"
        class="mb-4"
      >
        <h3>{{ $gettext('Custom') }}</h3>
        <CodeEditor
          v-model:content="data.custom"
          default-height="150px"
        />
      </div>
      <DirectiveEditor
        v-if="data.directives"
        :directives="data.directives"
        readonly
      />
      <LocationEditor
        v-if="data.locations"
        :locations="data.locations"
        readonly
      />
    </AModal>
  </div>
</template>

<style lang="less" scoped>
:deep(.ant-list-item) {
  padding: 12px;
}

:deep(.ant-list-item:first-child) {
  padding-top: 0;
}
</style>
