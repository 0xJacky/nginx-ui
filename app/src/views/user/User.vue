<script setup lang="ts">
import type { User } from '@/api/user'
import { StdCurd } from '@uozi-admin/curd'
import user from '@/api/user'
import { useMFAManagement } from '@/components/TwoFA/useMFAManagement'
import userColumns from '@/views/user/userColumns'
import UserMFA from './UserMFA.vue'

const table = useTemplateRef('table')
const selectedUser = ref<User>()
const authorize = useMFAManagement()

async function beforeSave(data: Record<string, unknown>) {
  if (typeof data.mfa_required !== 'boolean')
    return true
  const previous = data.id ? (await user.getItem(String(data.id))).mfa_required : false
  return previous === data.mfa_required || await authorize()
}
</script>

<template>
  <StdCurd
    ref="table"
    :before-save="beforeSave"
    :scroll-x="1000"
    :title="$gettext('Manage Users')"
    :columns="userColumns"
    disable-export
    :api="user"
  >
    <template #beforeActions="{ record }">
      <AButton type="link" size="small" @click="selectedUser = record">
        {{ $gettext('MFA Settings') }}
      </AButton>
    </template>
  </StdCurd>
  <AModal :open="!!selectedUser" :title="$gettext('MFA Settings')" :footer="null" destroy-on-close @cancel="selectedUser = undefined">
    <UserMFA v-if="selectedUser" :key="selectedUser.id" :record="selectedUser" @saved="table?.refresh()" />
  </AModal>
</template>

<style scoped>

</style>
