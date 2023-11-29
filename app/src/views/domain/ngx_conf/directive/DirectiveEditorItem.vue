<script setup lang="ts">
import { DeleteOutlined, HolderOutlined } from '@ant-design/icons-vue'

import { useGettext } from 'vue3-gettext'
import { ref, watch } from 'vue'
import { message } from 'ant-design-vue'
import config from '@/api/config'
import CodeEditor from '@/components/CodeEditor'
import type { NgxDirective } from '@/api/ngx'

const props = defineProps<{
  index: number
  readonly?: boolean
}>()

const { $gettext, interpolate } = useGettext()

const ngx_directives = inject('ngx_directives') as NgxDirective[]

function remove(index: number) {
  ngx_directives.splice(index, 1)
}

const content = ref('')

function init() {
  if (ngx_directives[props.index].directive === 'include') {
    config.get(ngx_directives[props.index].params).then(r => {
      content.value = r.content
    })
  }
}

watch(props, init)

function save() {
  config.save(ngx_directives[props.index].params, { content: content.value }).then(r => {
    content.value = r.content
    message.success($gettext('Saved successfully'))
  }).catch(r => {
    message.error(interpolate($gettext('Save error %{msg}'), { msg: r.message ?? '' }))
  })
}

const currentIdx = inject('current_idx')
</script>

<template>
  <div
    v-if="ngx_directives[props.index]"
    class="dir-editor-item"
  >
    <div class="input-wrapper">
      <div
        v-if="ngx_directives[props.index].directive === ''"
        class="code-editor-wrapper"
      >
        <HolderOutlined style="padding: 5px" />
        <CodeEditor
          v-model:content="ngx_directives[props.index].params"
          default-height="100px"
          style="width: 100%;"
        />
      </div>

      <AInput
        v-else
        v-model:value="ngx_directives[props.index].params"
        @click="currentIdx = index"
      >
        <template #addonBefore>
          <HolderOutlined />
          {{ ngx_directives[props.index].directive }}
        </template>
      </AInput>

      <APopconfirm
        v-if="!readonly"
        :title="$gettext('Are you sure you want to remove this directive?')"
        :ok-text="$gettext('Yes')"
        :cancel-text="$gettext('No')"
        @confirm="remove(index)"
      >
        <AButton>
          <template #icon>
            <DeleteOutlined style="font-size: 14px;" />
          </template>
        </AButton>
      </APopconfirm>
    </div>
    <div
      v-if="currentIdx === index"
      class="directive-editor-extra"
    >
      <div class="extra-content">
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Comments')">
            <ATextarea v-model:value="ngx_directives[props.index].comments" />
          </AFormItem>
          <AFormItem
            v-if="ngx_directives[props.index].directive === 'include'"
            :label="$gettext('Content')"
          >
            <CodeEditor
              v-model:content="content"
              default-height="200px"
              style="width: 100%;"
            />
            <div class="save-btn">
              <AButton @click="save">
                {{ $gettext('Save') }}
              </AButton>
            </div>
          </AFormItem>
        </AForm>
      </div>
    </div>
  </div>
</template>

<style lang="less" scoped>
.dir-editor-item {
  margin: 15px 0;
}

.code-editor-wrapper {
  display: flex;
  width: 100%;
  align-items: center;
}

.anticon-holder {
  cursor: grab;
}

.directive-editor-extra {
  background-color: #fafafa;
  padding: 10px 20px;
  margin-bottom: 10px;

  .save-btn {
    display: flex;
    justify-content: flex-end;
    margin-top: 15px;
  }
}

.input-wrapper {
  display: flex;
  gap: 10px;
  align-items: center;
}
</style>
