<script setup lang="ts">
import { useGettext } from 'vue3-gettext'
import { reactive, ref } from 'vue'
import { DeleteOutlined, HolderOutlined } from '@ant-design/icons-vue'
import Draggable from 'vuedraggable'
import CodeEditor from '@/components/CodeEditor'
import type { NgxConfig, NgxLocation } from '@/api/ngx'

const props = defineProps<{
  locations?: NgxLocation[]
  readonly?: boolean
  currentServerIndex?: number
}>()

const ngx_config = inject('ngx_config') as NgxConfig

const { $gettext } = useGettext()

const location = reactive({
  comments: '',
  path: '',
  content: '',
})

const adding = ref(false)

function add() {
  adding.value = true
  location.comments = ''
  location.path = ''
  location.content = ''
}

function save() {
  adding.value = false
  ngx_config.servers[props.currentServerIndex].locations?.push({
    ...location,
  })
}

function remove(index: number) {
  ngx_config.servers[props.currentServerIndex].locations?.splice(index, 1)
}
</script>

<template>
  <h2>
    {{ $gettext('Locations') }}
  </h2>
  <AEmpty v-if="!locations" />
  <Draggable
    v-else
    :list="locations"
    item-key="name"
    class="list-group"
    ghost-class="ghost"
    handle=".ant-collapse-header"
  >
    <template #item="{ element: v, index }">
      <ACollapse :bordered="false">
        <ACollapsePanel>
          <template #header>
            <div>
              <HolderOutlined />
              {{ $gettext('Location') }}
              {{ v.path }}
            </div>
          </template>
          <template
            v-if="!readonly"
            #extra
          >
            <APopconfirm
              :title="$gettext('Are you sure you want to remove this location?')"
              :ok-text="$gettext('Yes')"
              :cancel-text="$gettext('No')"
              @confirm="remove(index)"
            >
              <AButton
                type="text"
                size="small"
              >
                <template #icon>
                  <DeleteOutlined style="font-size: 14px;" />
                </template>
              </AButton>
            </APopconfirm>
          </template>
          <AForm layout="vertical">
            <AFormItem :label="$gettext('Comments')">
              <ATextarea
                v-model:value="v.comments"
                :bordered="false"
              />
            </AFormItem>
            <AFormItem :label="$gettext('Path')">
              <AInput
                v-model:value="v.path"
                addon-before="location"
              />
            </AFormItem>
            <AFormItem :label="$gettext('Content')">
              <CodeEditor
                v-model:content="v.content"
                default-height="200px"
                style="width: 100%;"
              />
            </AFormItem>
          </AForm>
        </ACollapsePanel>
      </ACollapse>
    </template>
  </Draggable>

  <AModal
    v-model:open="adding"
    :title="$gettext('Add Location')"
    @ok="save"
  >
    <AForm layout="vertical">
      <AFormItem :label="$gettext('Comments')">
        <ATextarea v-model:value="location.comments" />
      </AFormItem>
      <AFormItem :label="$gettext('Path')">
        <AInput
          v-model:value="location.path"
          addon-before="location"
        />
      </AFormItem>
      <AFormItem :label="$gettext('Content')">
        <CodeEditor
          v-model:content="location.content"
          default-height="200px"
        />
      </AFormItem>
    </AForm>
  </AModal>

  <div v-if="!readonly">
    <AButton
      block
      @click="add"
    >
      {{ $gettext('Add Location') }}
    </AButton>
  </div>
</template>

<style lang="less" scoped>
.ant-collapse {
  margin: 10px 0;
}

.ant-collapse-item {
  border: 0 !important;
}

.ant-collapse-header {
  align-items: center;
}
</style>
