<script setup lang="ts">
import type { NgxDirective } from '@/api/ngx'
import { DeleteOutlined, HolderOutlined, InfoCircleOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import config from '@/api/config'
import CodeEditor from '@/components/CodeEditor'
import { Include } from '..'
import DirectiveDocuments from './DirectiveDocuments.vue'
import { useDirectiveStore } from './store'

const props = defineProps<{
  index: number
  readonly?: boolean
  context?: string
}>()

const emit = defineEmits(['remove'])

const directiveStore = useDirectiveStore()
const { curIdx } = storeToRefs(directiveStore)

const directive = defineModel<NgxDirective>('directive', {
  default: reactive({}),
})

const content = ref('')

const shouldLoadInclude = computed(() => {
  return directive.value.directive === Include && !directive.value.params.includes('*')
})

function init() {
  // if directive is Include and params is not * #1278
  if (shouldLoadInclude.value) {
    config.getItem(directive.value.params).then(r => {
      content.value = r.content
    })
  }
}

init()

watch(props, init)

function save() {
  config.updateItem(directive.value.params, { content: content.value }).then(r => {
    content.value = r.content
    message.success($gettext('Saved successfully'))
  })
}

const onHover = ref(false)
const showComment = ref(false)
</script>

<template>
  <div
    v-if="directive"
    class="dir-editor-item"
  >
    <div class="input-wrapper" @mouseenter="onHover = true" @mouseleave="onHover = false">
      <div
        v-if="directive.directive === ''"
        class="code-editor-wrapper"
      >
        <HolderOutlined class="pa-2" />
        <CodeEditor
          v-model:content="directive.params"
          default-height="100px"
          class="w-full"
        />
      </div>

      <AInput
        v-else
        v-model:value="directive.params"
        @click="curIdx = index"
      >
        <template #addonBefore>
          <HolderOutlined />
          {{ directive.directive }}
        </template>
        <template #suffix>
          <slot
            name="suffix"
            :directive="directive"
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
        @confirm="emit('remove')"
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
            <ATextarea v-model:value="directive.comments" />
          </AFormItem>
          <AFormItem
            v-if="shouldLoadInclude"
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
          <DirectiveDocuments
            :directive="directive.directive"
          />
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
  transition: all .16s ease-in-out;
}

.fade-enter-from, .fade-enter-to, .fade-leave-to
  /* .fade-leave-active for below version 2.1.8 */ {
  opacity: 0;
}
</style>
