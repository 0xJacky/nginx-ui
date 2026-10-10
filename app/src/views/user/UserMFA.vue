<script setup lang="ts">
import type { User } from '@/api/user'
import user from '@/api/user'
import { useMFAManagement } from '@/components/TwoFA/useMFAManagement'

const props = defineProps<{ record: User }>()
const emit = defineEmits<{ saved: [] }>()
const { message } = useGlobalApp()
const authorize = useMFAManagement()
const detail = ref<User>(props.record)
const required = ref(props.record.mfa_required)
const loading = ref(false)

async function refresh() {
  detail.value = await user.getItem(props.record.id)
  required.value = detail.value.mfa_required
}

async function savePolicy() {
  if (!await authorize())
    return
  loading.value = true
  try {
    await user.updateItem(props.record.id, { mfa_required: required.value })
    await refresh()
    emit('saved')
    message.success($gettext('MFA policy updated. It takes effect at the next sign-in.'))
  }
  finally {
    loading.value = false
  }
}

async function resetMFA() {
  if (!await authorize())
    return
  loading.value = true
  try {
    await user.resetMFA(props.record.id)
    await refresh()
    emit('saved')
    message.success($gettext('MFA credentials and sessions reset'))
  }
  finally {
    loading.value = false
  }
}

onMounted(() => {
  void refresh()
})
</script>

<template>
  <AFlex vertical gap="middle">
    <ADescriptions :column="1" bordered size="small">
      <ADescriptionsItem :label="$gettext('Username')">
        {{ detail.name }}
      </ADescriptionsItem>
      <ADescriptionsItem :label="$gettext('MFA Enrollment')">
        {{ detail.mfa_pending ? $gettext('Pending enrollment') : detail.enabled_2fa ? $gettext('Enabled') : $gettext('Disabled') }}
      </ADescriptionsItem>
    </ADescriptions>
    <AAlert v-if="detail.mfa_policy_source === 'global'" type="info" show-icon :title="$gettext('Global MFA enforcement is active. Changing the user policy does not override it.')" />
    <AForm layout="vertical">
      <AFormItem :label="$gettext('Require MFA for this user')">
        <AFlex align="center" gap="middle">
          <ASwitch v-model:checked="required" :disabled="loading" />
          <AButton type="primary" :loading :disabled="required === detail.mfa_required" @click="savePolicy">
            {{ $gettext('Save') }}
          </AButton>
        </AFlex>
      </AFormItem>
    </AForm>
    <AAlert type="warning" show-icon :title="$gettext('Reset removes TOTP, all passkeys and recovery codes, and ends all sessions. The MFA requirement is preserved.')" />
    <APopconfirm :title="$gettext('Reset all MFA credentials and sessions for this user?')" @confirm="resetMFA">
      <AButton danger :loading>
        {{ $gettext('Reset MFA') }}
      </AButton>
    </APopconfirm>
  </AFlex>
</template>
