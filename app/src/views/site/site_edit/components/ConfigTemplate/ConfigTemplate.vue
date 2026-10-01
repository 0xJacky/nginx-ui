<script setup lang="ts">
import type { Template } from '@/api/template'
import { SearchOutlined } from '@antdv-next/icons'
import { storeToRefs } from 'pinia'
import template from '@/api/template'
import CodeEditor from '@/components/CodeEditor'
import { List, ListItem, ListItemMeta } from '@/components/List'
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

type Source = 'all' | 'custom' | 'builtin'
const source = ref<Source>('all')

function getBlockList() {
  template.get_block_list().then(r => {
    blocks.value = r.data
  })
}

getBlockList()

function view(item: Template) {
  visible.value = true
  name.value = item.filename
  template.get_block(item.filename, item.origin).then(r => {
    data.value = r
  })
}

function isCustom(item: Template) {
  return item.origin === 'custom'
}

const customCount = computed(() => blocks.value.filter(isCustom).length)

const sourceOptions = computed(() => [
  { label: `${$gettext('All')} ${blocks.value.length}`, value: 'all' },
  { label: `${$gettext('Snippets')} ${customCount.value}`, value: 'custom' },
  { label: `${$gettext('Built-in')} ${blocks.value.length - customCount.value}`, value: 'builtin' },
])

// A snippet without variables is the same file on every use, so a site can
// include it and follows its changes. One with variables is filled in here
// and can only be copied into the site.
const canInclude = computed(() => isCustom(data.value) && Object.keys(data.value.variables ?? {}).length === 0)

function displayName(item: Template) {
  return item.name_i18n?.[language.value] || item.name || item.filename
}

const transDescription = computed(() => {
  return (item: { description: { [key: string]: string } }) =>
    item.description?.[language.value] ?? item.description?.en ?? ''
})

// The snippets of the user come first: they are what a site most likely
// reuses, and there are fewer of them than built-in templates.
const filteredBlocks = computed(() => {
  const searchText = filterText.value.toLowerCase()
  return blocks.value
    .filter(item => source.value === 'all' || (source.value === 'custom') === isCustom(item))
    .filter(item => !searchText
      || displayName(item).toLowerCase().includes(searchText)
      || item.author?.toLowerCase().includes(searchText)
      || transDescription.value(item).toLowerCase().includes(searchText))
    .sort((a, b) => Number(isCustom(b)) - Number(isCustom(a)))
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

function include() {
  curServer.value?.directives?.push({ directive: 'include', params: `snippets/${data.value.filename}` })
  visible.value = false
}
</script>

<template>
  <div>
    <div class="mb-4 flex flex-col gap-3">
      <AInput
        v-model:value="filterText"
        :placeholder="$gettext('Search templates')"
        allow-clear
      >
        <template #prefix>
          <SearchOutlined />
        </template>
      </AInput>
      <ASegmented
        v-if="customCount > 0"
        v-model:value="source"
        :options="sourceOptions"
        block
      />
    </div>
    <div class="config-list-wrapper">
      <List :data-source="filteredBlocks">
        <template #renderItem="{ item }">
          <ListItem>
            <ListItemMeta>
              <template #title>
                <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
                  <span>{{ displayName(item) }}</span>
                  <ATag
                    class="m-0"
                    :color="isCustom(item) ? 'green' : 'default'"
                    :bordered="false"
                  >
                    {{ isCustom(item) ? $gettext('Snippet') : $gettext('Built-in') }}
                  </ATag>
                </div>
              </template>
              <template #description>
                <p
                  v-if="item.author"
                  class="mt-4"
                >
                  {{ $gettext('Author') }}: {{ item.author }}
                </p>
                <p
                  v-if="transDescription(item)"
                  class="mb-0"
                  :class="{ 'mt-4': !item.author }"
                >
                  {{ $gettext('Description') }}: {{ transDescription(item) }}
                </p>
              </template>
            </ListItemMeta>
            <template #actions>
              <AButton
                type="link"
                @click="view(item)"
              >
                {{ $gettext('View') }}
              </AButton>
            </template>
          </ListItem>
        </template>
      </List>
    </div>
    <AModal
      v-model:open="visible"
      :title="data.name_i18n?.[language] || data.name"
      :mask="false"
    >
      <AAlert
        v-if="canInclude"
        class="mb-4"
        type="info"
        show-icon
        :title="$gettext('Include keeps the site linked to snippets/%{file}, so later changes of the snippet apply to it. Insert copies the content into the site instead.', { file: data.filename })"
      />
      <p v-if="data.author">
        {{ $gettext('Author') }}: {{ data.author }}
      </p>
      <p v-if="transDescription(data)">
        {{ $gettext('Description') }}: {{ transDescription(data) }}
      </p>
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
      <template #footer>
        <AFlex justify="end" gap="small" wrap>
          <AButton @click="visible = false">
            {{ $gettext('Cancel') }}
          </AButton>
          <AButton
            :type="canInclude ? 'default' : 'primary'"
            @click="add"
          >
            {{ canInclude ? $gettext('Insert') : $gettext('Add') }}
          </AButton>
          <AButton
            v-if="canInclude"
            type="primary"
            @click="include"
          >
            {{ $gettext('Include') }}
          </AButton>
        </AFlex>
      </template>
    </AModal>
  </div>
</template>

<style lang="less" scoped>
:deep(.nui-list-item) {
  padding: 12px;
}

:deep(.nui-list-item:first-child) {
  padding-top: 0;
}
</style>
