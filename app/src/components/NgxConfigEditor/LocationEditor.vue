<script setup lang="ts">
import type { NgxLocation } from '@/api/ngx'
import { CopyOutlined, DeleteOutlined, HolderOutlined } from '@antdv-next/icons'
import { cloneDeep } from 'lodash'
import Draggable from 'vuedraggable'
import CodeEditor from '@/components/CodeEditor'
import { useManagedUpstreams } from '@/composables/useManagedUpstreams'
import { getProxyPassUpstream, setProxyPassUpstream } from './proxyPassUpstream'

defineProps<{
  readonly?: boolean
}>()

const locations = defineModel<NgxLocation[]>('locations', {
  default: () => [],
})

const locationKeys = new WeakMap<NgxLocation, string>()
let locationKeySeed = 0

const location = reactive({
  comments: '',
  path: '',
  content: '',
})

const adding = ref(false)

// Upstream groups managed from the Upstream page; a location can proxy to one
// by name instead of repeating the backend addresses in every site.
const { options: upstreamOptions, names: upstreamNames } = useManagedUpstreams()

function locationUpstream(loc: { content: string }) {
  return getProxyPassUpstream(loc.content ?? '', upstreamNames.value)
}

function setLocationUpstream(loc: { content: string }, name: unknown) {
  if (typeof name === 'string' && name)
    loc.content = setProxyPassUpstream(loc.content ?? '', name)
}

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

const Space = resolveComponent('ASpace')
const Button = resolveComponent('AButton')
const Popconfirm = resolveComponent('APopconfirm')

function getLocationExtra(index: number) {
  return h(
    Space,
    {},
    {
      default: () => [
        h(
          Button,
          {
            type: 'text',
            size: 'small',
            onClick: () => duplicate(index),
          },
          {
            icon: () => h(CopyOutlined, { style: 'font-size: 14px;' }),
          },
        ),
        h(
          Popconfirm,
          {
            title: $gettext('Are you sure you want to remove this location?'),
            okText: $gettext('Yes'),
            cancelText: $gettext('No'),
            onConfirm: () => remove(index),
          },
          {
            default: () => h(
              Button,
              {
                type: 'text',
                size: 'small',
              },
              {
                icon: () => h(DeleteOutlined, { style: 'font-size: 14px;' }),
              },
            ),
          },
        ),
      ],
    },
  )
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
          :items="[{
            key: getLocationKey(v),
            style: { border: '0' },
            extra: !readonly ? getLocationExtra(index) : undefined,
          }]"
          :styles="{
            root: { margin: '10px 0' },
            header: { alignItems: 'center' },
            title: { maxWidth: 'calc(90% - 56px)' },
          }"
        >
          <template #labelRender>
            <HolderOutlined />
            {{ $gettext('Location') }}
            {{ v.path }}
            <slot name="label-extra" :location="v" :index="index" />
          </template>
          <template #contentRender>
            <AForm layout="vertical">
              <AFormItem :label="$gettext('Comments')">
                <ATextarea
                  v-model:value="v.comments"
                  variant="borderless"
                />
              </AFormItem>
              <AFormItem :label="$gettext('Path')">
                <ASpaceCompact block>
                  <ASpaceAddon>location</ASpaceAddon>
                  <AInput v-model:value="v.path" />
                </ASpaceCompact>
              </AFormItem>
              <slot name="form-extra" :location="v" :index="index" />
              <AFormItem
                v-if="upstreamOptions.length > 0 && !readonly"
                :label="$gettext('Proxy to upstream')"
                :extra="$gettext('Sets proxy_pass to the selected upstream group; its servers are managed on the Upstream page.')"
              >
                <ASelect
                  class="w-full max-w-100"
                  :value="locationUpstream(v)"
                  :options="upstreamOptions"
                  :placeholder="$gettext('Select an upstream')"
                  data-testid="location-upstream-select"
                  @change="name => setLocationUpstream(v, name)"
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
          </template>
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
          <ASpaceCompact block>
            <ASpaceAddon>location</ASpaceAddon>
            <AInput v-model:value="location.path" />
          </ASpaceCompact>
        </AFormItem>
        <AFormItem
          v-if="upstreamOptions.length > 0"
          :label="$gettext('Proxy to upstream')"
        >
          <ASelect
            class="w-full max-w-100"
            :value="locationUpstream(location)"
            :options="upstreamOptions"
            :placeholder="$gettext('Select an upstream')"
            @change="name => setLocationUpstream(location, name)"
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
