<script setup lang="tsx">
import type { EnvGroup } from '@/api/env_group'
import { StdCurd } from '@uozi-admin/curd'
import { message } from 'ant-design-vue'
import env_group from '@/api/env_group'
import stream from '@/api/stream'
import EnvGroupTabs from '@/components/EnvGroupTabs'
import InspectConfig from '@/views/config/InspectConfig.vue'
import columns from '@/views/stream/columns'
import StreamDuplicate from '@/views/stream/components/StreamDuplicate.vue'

const route = useRoute()
const router = useRouter()

const curd = ref()
const inspect_config = ref()

const envGroupId = ref(Number.parseInt(route.query.env_group_id as string) || 0)
const envGroups = ref<EnvGroup[]>([])

onMounted(async () => {
  let page = 1
  while (true) {
    try {
      const { data, pagination } = await env_group.getList({ page })
      if (!data || !pagination)
        return
      envGroups.value.push(...data)
      if (data.length < pagination?.per_page) {
        return
      }
      page++
    }
    catch {
      return
    }
  }
})

watch(route, () => {
  inspect_config.value?.test()
})

function enable(name: string) {
  stream.enable(name).then(() => {
    message.success($gettext('Enabled successfully'))
    curd.value?.refresh()
    inspect_config.value?.test()
  }).catch(r => {
    message.error($gettext('Failed to enable %{msg}', { msg: r.message ?? '' }), 10)
  })
}

function disable(name: string) {
  stream.disable(name).then(() => {
    message.success($gettext('Disabled successfully'))
    curd.value?.refresh()
    inspect_config.value?.test()
  }).catch(r => {
    message.error($gettext('Failed to disable %{msg}', { msg: r.message ?? '' }))
  })
}

function destroy(stream_name: string) {
  stream.deleteItem(stream_name).then(() => {
    curd.value.refresh()
    message.success($gettext('Delete stream: %{stream_name}', { stream_name }))
    inspect_config.value?.test()
  })
}

const showDuplicator = ref(false)
const target = ref('')

function handle_click_duplicate(name: string) {
  showDuplicator.value = true
  target.value = name
}

const showAddStream = ref(false)
const name = ref('')

function add() {
  showAddStream.value = true
  name.value = ''
}

function handleAddStream() {
  stream.updateItem(name.value, { name: name.value, content: 'server\t{\n\n}' }).then(() => {
    showAddStream.value = false
    curd.value?.refresh()
    message.success($gettext('Added successfully'))
  })
}
</script>

<template>
  <div>
    <StdCurd
      ref="curd"
      :title="$gettext('Manage Streams')"
      :api="stream"
      :columns="columns"
      :table-props="{
        rowKey: 'name',
      }"
      disable-delete
      disable-view
      row-selection-type="checkbox"
      :custom-query-params="{
        env_group_id: envGroupId,
      }"
      :scroll-x="800"
      @edit-item="record => router.push({
        path: `/streams/${encodeURIComponent(record.name)}`,
      })"
    >
      <template #extra>
        <div class="flex items-center cursor-default">
          <a class="mr-4" @click="add">{{ $gettext('Add') }}</a>
        </div>
      </template>

      <template #beforeCardBody>
        <InspectConfig ref="inspect_config" />
        <EnvGroupTabs v-model:active-key="envGroupId" :env-groups="envGroups" />
      </template>

      <template #afterActions="{ record }">
        <AButton
          v-if="record.enabled"
          type="link"
          size="small"
          @click="disable(record.name)"
        >
          {{ $gettext('Disable') }}
        </AButton>
        <AButton
          v-else
          type="link"
          size="small"
          @click="enable(record.name)"
        >
          {{ $gettext('Enable') }}
        </AButton>
        <AButton
          type="link"
          size="small"
          @click="handle_click_duplicate(record.name)"
        >
          {{ $gettext('Duplicate') }}
        </AButton>
        <APopconfirm
          :cancel-text="$gettext('No')"
          :ok-text="$gettext('OK')"
          :title="$gettext('Are you sure you want to delete?')"
          :disabled="record.enabled"
          @confirm="destroy(record.name)"
        >
          <AButton
            type="link"
            size="small"
            :disabled="record.enabled"
          >
            {{ $gettext('Delete') }}
          </AButton>
        </APopconfirm>
      </template>
    </StdCurd>

    <AModal
      v-model:open="showAddStream"
      :title="$gettext('Add Stream')"
      :mask="false"
      @ok="handleAddStream"
    >
      <AForm layout="vertical">
        <AFormItem :label="$gettext('Name')">
          <AInput v-model:value="name" />
        </AFormItem>
      </AForm>
    </AModal>

    <StreamDuplicate
      v-model:visible="showDuplicator"
      :name="target"
      @duplicated="() => curd.refresh()"
    />
  </div>
</template>

<style scoped>

</style>
