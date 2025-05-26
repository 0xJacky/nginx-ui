<script setup lang="tsx">
import type { TwoFAStatus } from '@/api/2fa'
import type { RecoveryCode } from '@/api/recovery'
import { message } from 'ant-design-vue'
import twoFA from '@/api/2fa'
import { use2FAModal } from '@/components/TwoFA'
import { useUserStore } from '@/pinia'
import { Passkey, RecoveryCodes, TOTP } from '@/views/preference/components/AuthSettings'

const twoFAStatus = ref<TwoFAStatus>({} as TwoFAStatus)
const recoveryCodes = ref<RecoveryCode[]>()

const userStore = useUserStore()
const { info } = storeToRefs(userStore)

// Form data
const userForm = ref({
  name: '',
})

const passwordForm = ref({
  old_password: '',
  new_password: '',
  confirm_password: '',
})

const loading = ref(false)
const passwordLoading = ref(false)

function get2FAStatus() {
  twoFA.status().then(r => {
    twoFAStatus.value = r
  })
}

async function getCurrentUser() {
  try {
    loading.value = true
    await userStore.getCurrentUser()
    // Update form with current user data
    userForm.value.name = info.value.name || ''
  }
  catch (error) {
    console.error('Failed to get current user:', error)
    // Handle error (could show notification)
  }
  finally {
    loading.value = false
  }
}

async function updateUserInfo() {
  try {
    loading.value = true
    const otpModal = use2FAModal()

    otpModal.open().then(() => {
      userStore.updateCurrentUser({
        name: userForm.value.name,
      })
      // Show success message
      message.success($gettext('User info updated successfully'))
    })
  }
  catch (error) {
    console.error('Failed to update user info:', error)
  }
  finally {
    loading.value = false
  }
}

async function changePassword() {
  if (passwordForm.value.new_password !== passwordForm.value.confirm_password) {
    message.error($gettext('Passwords do not match'))
    return
  }

  try {
    passwordLoading.value = true
    const otpModal = use2FAModal()

    otpModal.open().then(async () => {
      await userStore.updateCurrentUserPassword({
        old_password: passwordForm.value.old_password,
        new_password: passwordForm.value.new_password,
      })

      // Clear password form
      passwordForm.value = {
        old_password: '',
        new_password: '',
        confirm_password: '',
      }

      message.success($gettext('Password updated successfully'))
    })
  }
  catch (error) {
    console.error('Failed to update password:', error)
    // Handle error (could show notification)
  }
  finally {
    passwordLoading.value = false
  }
}

// Initialize data on mount
onMounted(() => {
  getCurrentUser()
  get2FAStatus()
})
</script>

<template>
  <div>
    <div class="max-w-4xl mx-auto">
      <!-- Personal Information Section -->
      <div class="mb-8">
        <h2 class="text-xl font-semibold mb-4">
          {{ $gettext('Personal Information') }}
        </h2>
        <ACard class="mb-4">
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label class="block text-sm font-medium mb-2">
                {{ $gettext('Username') }}
              </label>
              <AInput
                v-model:value="userForm.name"
                :placeholder="$gettext('Username')"
                class="w-full"
              />
            </div>
          </div>
          <div class="mt-4">
            <AButton
              type="primary"
              :loading="loading"
              @click="updateUserInfo"
            >
              {{ $gettext('Update Profile') }}
            </AButton>
          </div>
        </ACard>
      </div>

      <!-- 2FA Settings Section -->
      <div class="mb-8">
        <h2 class="text-xl font-semibold mb-4">
          {{ $gettext('2FA Settings') }}
        </h2>
        <ACard>
          <Passkey class="mb-4" />

          <TOTP
            v-model:recovery-codes="recoveryCodes"
            class="mb-4"
            :status="twoFAStatus?.otp_status"
            @refresh="get2FAStatus"
          />

          <RecoveryCodes
            class="mb-4"
            :two-f-a-status="twoFAStatus"
            :recovery-codes="recoveryCodes"
            @refresh="get2FAStatus"
          />
        </ACard>
      </div>

      <!-- Security Settings Section -->
      <div class="mb-8">
        <h2 class="text-xl font-semibold mb-4">
          {{ $gettext('Security Settings') }}
        </h2>
        <ACard>
          <div class="space-y-4">
            <div>
              <h3 class="text-lg font-medium mb-2">
                {{ $gettext('Change Password') }}
              </h3>
              <div class="grid grid-cols-1 gap-4">
                <div>
                  <label class="block text-sm font-medium mb-2">
                    {{ $gettext('Current Password') }}
                  </label>
                  <AInputPassword
                    v-model:value="passwordForm.old_password"
                    class="w-full max-w-xs"
                  />
                </div>
                <div>
                  <label class="block text-sm font-medium mb-2">
                    {{ $gettext('New Password') }}
                  </label>
                  <AInputPassword
                    v-model:value="passwordForm.new_password"
                    class="w-full max-w-xs"
                  />
                </div>
                <div>
                  <label class="block text-sm font-medium mb-2">
                    {{ $gettext('Confirm New Password') }}
                  </label>
                  <AInputPassword
                    v-model:value="passwordForm.confirm_password"
                    class="w-full max-w-xs"
                  />
                </div>
                <div>
                  <AButton
                    type="primary"
                    :loading="passwordLoading"
                    :disabled="!passwordForm.old_password || !passwordForm.new_password || !passwordForm.confirm_password"
                    @click="changePassword"
                  >
                    {{ $gettext('Update Password') }}
                  </AButton>
                </div>
              </div>
            </div>
          </div>
        </ACard>
      </div>
    </div>
  </div>
</template>

<style lang="less" scoped>
</style>
