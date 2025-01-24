<script setup lang="ts">
import type { Passkey } from '@/api/passkey'
import passkey from '@/api/passkey'
import ReactiveFromNow from '@/components/ReactiveFromNow/ReactiveFromNow.vue'
import { formatDateTime } from '@/lib/helper'
import { useUserStore } from '@/pinia'
import AddPasskey from '@/views/preference/components/AddPasskey.vue'
import { DeleteOutlined, EditOutlined, KeyOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'

dayjs.extend(relativeTime)

const user = useUserStore()

const getListLoading = ref(true)
const data = ref([]) as Ref<Passkey[]>
const passkeyName = ref('')

function getList() {
  getListLoading.value = true
  passkey.get_list().then(r => {
    data.value = r
  }).finally(() => {
    getListLoading.value = false
  })
}

onMounted(() => {
  getList()
})

const modifyIdx = ref(-1)
function update(id: number, record: Passkey) {
  passkey.update(id, record).then(() => {
    getList()
    modifyIdx.value = -1
    message.success($gettext('Update successfully'))
  })
}

function remove(item: Passkey) {
  passkey.remove(item.id).then(() => {
    getList()
    message.success($gettext('Remove successfully'))

    // if current passkey is removed, clear it from user store
    if (user.passkeyLoginAvailable && user.passkeyRawId === item.raw_id)
      user.passkeyRawId = ''
  })
}
</script>

<template>
  <div>
    <div>
      <h3>
        {{ $gettext('Passkey') }}
      </h3>
      <p>
        {{ $gettext('Passkeys are webauthn credentials that validate your identity using touch, '
          + 'facial recognition, a device password, or a PIN. '
          + 'They can be used as a password replacement or as a 2FA method.') }}
      </p>
    </div>
    <AList
      class="mt-4"
      bordered
      :data-source="data"
    >
      <template #header>
        <div class="flex items-center justify-between">
          <div class="font-bold">
            {{ $gettext('Your passkeys') }}
          </div>
          <AddPasskey @created="() => getList()" />
        </div>
      </template>
      <template #renderItem="{ item, index }">
        <AListItem>
          <AListItemMeta>
            <template #title>
              <div class="flex gap-2">
                <KeyOutlined />
                <div v-if="index !== modifyIdx">
                  {{ item.name }}
                </div>
                <div v-else>
                  <AInput v-model:value="passkeyName" />
                </div>
              </div>
            </template>
            <template #description>
              {{ $gettext('Created at') }}: {{ formatDateTime(item.created_at) }} · {{
                $gettext('Last used at') }}: <ReactiveFromNow :time="item.last_used_at" />
            </template>
          </AListItemMeta>
          <template #extra>
            <div v-if="modifyIdx !== index">
              <AButton
                type="link"
                size="small"
                @click="() => {
                  modifyIdx = index
                  passkeyName = item.name
                }"
              >
                <EditOutlined />
              </AButton>

              <APopconfirm
                :title="$gettext('Are you sure to delete this passkey immediately?')"
                @confirm="() => remove(item)"
              >
                <AButton
                  type="link"
                  danger
                  size="small"
                >
                  <DeleteOutlined />
                </AButton>
              </APopconfirm>
            </div>
            <div v-else>
              <AButton
                size="small"
                @click="() => update(item.id, { ...item, name: passkeyName })"
              >
                {{ $gettext('Save') }}
              </AButton>

              <AButton
                type="link"
                size="small"
                @click="() => {
                  modifyIdx = -1
                  passkeyName = item.name
                }"
              >
                {{ $gettext('Cancel') }}
              </AButton>
            </div>
          </template>
        </AListItem>
      </template>
    </AList>
  </div>
</template>

<style scoped lang="less">

</style>
