<script setup lang="ts">
import type { NgxDirective } from '@/api/ngx'
import config from '@/api/config'
import CodeEditor from '@/components/CodeEditor'
import { DeleteOutlined, HolderOutlined, InfoCircleOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'

const props = defineProps<{
  index: number
  readonly?: boolean
  context?: string
}>()

const ngxDirectives = inject('ngx_directives') as ComputedRef<NgxDirective[]>

function remove(index: number) {
  ngxDirectives.value.splice(index, 1)
}

const content = ref('')

function init() {
  if (ngxDirectives.value[props.index].directive === 'include') {
    config.get(ngxDirectives.value[props.index].params).then(r => {
      content.value = r.content
    })
  }
}

init()

watch(props, init)

function save() {
  config.save(ngxDirectives.value[props.index].params, { content: content.value }).then(r => {
    content.value = r.content
    message.success($gettext('Saved successfully'))
  }).catch(r => {
    message.error($gettext('Save error %{msg}', { msg: r.message ?? '' }))
  })
}

const currentIdx = inject<Ref<number>>('current_idx')!

const onHover = ref(false)
const showComment = ref(false)
</script>

<template>
  <div
    v-if="ngxDirectives[props.index]"
    class="dir-editor-item"
  >
    <div class="input-wrapper" @mouseenter="onHover = true" @mouseleave="onHover = false">
      <div
        v-if="ngxDirectives[props.index].directive === ''"
        class="code-editor-wrapper"
      >
        <HolderOutlined class="pa-2" />
        <CodeEditor
          v-model:content="ngxDirectives[props.index].params"
          default-height="100px"
          class="w-full"
        />
      </div>

      <AInput
        v-else
        v-model:value="ngxDirectives[props.index].params"
        @click="currentIdx = index"
      >
        <template #addonBefore>
          <HolderOutlined />
          {{ ngxDirectives[props.index].directive }}
        </template>
        <template #suffix>
          <slot
            name="suffix"
            :directive="ngxDirectives[props.index]"
          />

          <!-- Comments Entry -->
          <Transition name="fade">
            <div v-show="onHover" class="ml-3 cursor-pointer" @click="showComment = !showComment">
              <InfoCircleOutlined />
            </div>
          </Transition>
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
      v-if="showComment"
      class="directive-editor-extra"
    >
      <div class="extra-content">
        <AForm layout="vertical">
          <AFormItem :label="$gettext('Comments')">
            <ATextarea v-model:value="ngxDirectives[props.index].comments" />
          </AFormItem>
          <AFormItem
            v-if="ngxDirectives[props.index].directive === 'include'"
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
  border-radius: 5px;
  padding: 10px 20px;
  margin: 10px 0;

  .save-btn {
    display: flex;
    justify-content: flex-end;
    margin-top: 15px;
  }
}

.dark {
  .directive-editor-extra {
    background-color: #1f1f1f;
  }
}

.input-wrapper {
  display: flex;
  gap: 10px;
  align-items: center;
}

.fade-enter-active, .fade-leave-active {
  transition: all .2s ease-in-out;
}

.fade-enter-from, .fade-enter-to, .fade-leave-to
  /* .fade-leave-active for below version 2.1.8 */ {
  opacity: 0;
}
</style>
