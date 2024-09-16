<script setup lang="ts">
import { message } from 'ant-design-vue'
import { startRegistration } from '@simplewebauthn/browser'
import { DeleteOutlined, EditOutlined, KeyOutlined } from '@ant-design/icons-vue'
import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import { formatDateTime } from '@/lib/helper'
import type { Passkey } from '@/api/passkey'
import passkey from '@/api/passkey'
import ReactiveFromNow from '@/components/ReactiveFromNow/ReactiveFromNow.vue'
import { useUserStore } from '@/pinia'

dayjs.extend(relativeTime)

const user = useUserStore()
const passkeyName = ref('')
const addPasskeyModelOpen = ref(false)

const regLoading = ref(false)
async function registerPasskey() {
  regLoading.value = true
  try {
    const options = await passkey.begin_registration()

    const attestationResponse = await startRegistration(options.publicKey)

    await passkey.finish_registration(attestationResponse, passkeyName.value)

    getList()

    message.success($gettext('Register passkey successfully'))
    addPasskeyModelOpen.value = false

    user.passkeyRawId = attestationResponse.rawId
  }
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  catch (e: any) {
    message.error($gettext(e.message ?? 'Server error'))
  }
  regLoading.value = false
}

const getListLoading = ref(true)
const data = ref([]) as Ref<Passkey[]>

function getList() {
  getListLoading.value = true
  passkey.get_list().then(r => {
    data.value = r
  }).catch((e: { message?: string }) => {
    message.error(e?.message ?? $gettext('Server error'))
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
  }).catch((e: { message?: string }) => {
    message.error(e?.message ?? $gettext('Server error'))
  })
}

function remove(item: Passkey) {
  passkey.remove(item.id).then(() => {
    getList()
    message.success($gettext('Remove successfully'))

    // if current passkey is removed, clear it from user store
    if (user.passkeyLoginAvailable && user.passkeyRawId === item.raw_id)
      user.passkeyRawId = ''
  }).catch((e: { message?: string }) => {
    message.error(e?.message ?? $gettext('Server error'))
  })
}

function addPasskey() {
  addPasskeyModelOpen.value = true
  passkeyName.value = ''
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
          <AButton @click="addPasskey">
            {{ $gettext('Add a passkey') }}
          </AButton>
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
                @click="() => modifyIdx = index"
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

    <AModal
      v-model:open="addPasskeyModelOpen"
      :title="$gettext('Add a passkey')"
      centered
      :mask="false"
      :mask-closable="false"
      :closable="false"
      :confirm-loading="regLoading"
      @ok="registerPasskey"
    >
      <AForm layout="vertical">
        <AFormItem :label="$gettext('Name')">
          <AInput v-model:value="passkeyName" />
        </AFormItem>
      </AForm>
    </AModal>
  </div>
</template>

<style scoped lang="less">

</style>
