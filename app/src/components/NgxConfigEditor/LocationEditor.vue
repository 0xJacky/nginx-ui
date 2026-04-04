<script setup lang="ts">
import type { NgxLocation } from '@/api/ngx'
import { CopyOutlined, DeleteOutlined, HolderOutlined } from '@ant-design/icons-vue'
import { cloneDeep } from 'lodash'
import Draggable from 'vuedraggable'
import CodeEditor from '@/components/CodeEditor'

defineProps<{
  readonly?: boolean
}>()

const locations = defineModel<NgxLocation[]>('locations', {
  default: reactive([]),
})

const locationKeys = new WeakMap<NgxLocation, string>()
let locationKeySeed = 0

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
  locations.value.push({
    ...location,
  })
}

function remove(index: number) {
  locations.value.splice(index, 1)
}

function duplicate(index: number) {
  const loc = locations.value[index]

  locations.value.splice(index, 0, cloneDeep(loc))
}

function getLocationKey(location: NgxLocation) {
  let key = locationKeys.get(location)
  if (!key) {
    key = `location-${locationKeySeed++}`
    locationKeys.set(location, key)
  }
  return key
}
</script>

<template>
  <div>
    <h3>{{ $gettext('Locations') }}</h3>
    <AEmpty v-if="locations && locations?.length === 0" />
    <Draggable
      v-else
      :list="locations"
      :item-key="getLocationKey"
      class="list-group"
      ghost-class="ghost"
      handle=".ant-collapse-header"
    >
      <template #item="{ element: v, index }">
        <ACollapse
          :bordered="false"
          collapsible="header"
        >
          <ACollapsePanel>
            <template #header>
              <HolderOutlined />
              {{ $gettext('Location') }}
              {{ v.path }}
            </template>
            <template
              v-if="!readonly"
              #extra
            >
              <ASpace>
                <AButton
                  type="text"
                  size="small"
                  @click="() => duplicate(index)"
                >
                  <template #icon>
                    <CopyOutlined style="font-size: 14px;" />
                  </template>
                </AButton>
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
              </ASpace>
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

:deep(.ant-collapse-header-text) {
  max-width: calc(90% - 56px);
}
</style>
