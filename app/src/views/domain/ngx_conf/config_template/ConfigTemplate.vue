<script setup lang="ts">
import { useGettext } from 'vue3-gettext'
import { storeToRefs } from 'pinia'
import type { Ref } from 'vue'
import type { Template } from '@/api/template'
import template from '@/api/template'
import { useSettingsStore } from '@/pinia'

import DirectiveEditor from '@/views/domain/ngx_conf/directive/DirectiveEditor.vue'
import LocationEditor from '@/views/domain/ngx_conf/LocationEditor.vue'
import CodeEditor from '@/components/CodeEditor/CodeEditor.vue'
import TemplateForm from '@/views/domain/ngx_conf/config_template/TemplateForm.vue'
import type { NgxConfig } from '@/api/ngx'

const props = defineProps<{
  currentServerIndex: number
}>()

const { $gettext } = useGettext()
const { language } = storeToRefs(useSettingsStore())
const ngx_config = inject('ngx_config') as NgxConfig
const blocks = ref([])
const data = ref({}) as Ref<Template>
const visible = ref(false)
const name = ref('')

function get_block_list() {
  template.get_block_list().then(r => {
    blocks.value = r.data
  })
}

get_block_list()

function view(n: string) {
  visible.value = true
  name.value = n
  template.get_block(n).then(r => {
    data.value = r
  })
}

const trans_description = computed(() => {
  return (item: { description: { [key: string]: string } }) =>
    item.description?.[language.value] ?? item.description?.en ?? ''
})

async function add() {
  if (data.value.custom)
    ngx_config.custom += `\n${data.value.custom}`

  ngx_config.custom = ngx_config.custom?.trim()

  if (data.value.locations)
    ngx_config?.servers?.[props.currentServerIndex]?.locations?.push(...data.value.locations)

  if (data.value.directives)
    ngx_config?.servers?.[props.currentServerIndex]?.directives?.push(...data.value.directives)

  visible.value = false
}

const variables = computed(() => {
  return data.value.variables
})

function build_template() {
  template.build_block(name.value, variables.value).then(r => {
    data.value.directives = r.directives
    data.value.locations = r.locations
    data.value.custom = r.custom
  })
}

const ngx_directives = computed(() => {
  return data.value?.directives
})

provide('build_template', build_template)
provide('ngx_directives', ngx_directives)
</script>

<template>
  <div>
    <h3>
      {{ $gettext('Config Templates') }}
    </h3>
    <div class="config-list-wrapper">
      <AList
        :grid="{ gutter: 16, xs: 1, sm: 2, md: 2, lg: 2, xl: 2, xxl: 2, xxxl: 2 }"
        :data-source="blocks"
      >
        <template #renderItem="{ item }">
          <AListItem>
            <ACard
              size="small"
              :title="item.name"
            >
              <template #extra>
                <AButton
                  type="link"
                  size="small"
                  @click="view(item.filename)"
                >
                  {{ $gettext('View') }}
                </AButton>
              </template>
              <p>{{ $gettext('Author') }}: {{ item.author }}</p>
              <p>{{ $gettext('Description') }}: {{ trans_description(item) }}</p>
            </ACard>
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
      <p>{{ $gettext('Description') }}: {{ trans_description(data) }}</p>
      <TemplateForm v-model:data="data.variables" />
      <template v-if="data.custom">
        <h2>{{ $gettext('Custom') }}</h2>
        <CodeEditor
          v-model:content="data.custom"
          default-height="150px"
        />
      </template>
      <DirectiveEditor
        v-if="data.directives"
        readonly
      />
      <br>
      <LocationEditor
        v-if="data.locations"
        :locations="data.locations"
        readonly
      />
    </AModal>
  </div>
</template>

<style lang="less" scoped>
.config-list-wrapper {
  max-height: 200px;
  overflow-y: scroll;
  overflow-x: hidden;
}
</style>
