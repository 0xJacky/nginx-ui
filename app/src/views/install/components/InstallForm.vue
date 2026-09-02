<script setup lang="ts">
import type { FormInstance } from 'antdv-next'
import { LockOutlined, MailOutlined, UserOutlined } from '@antdv-next/icons'
import install from '@/api/install'

const props = defineProps<{
  installSecret: string
  frontendDebug?: boolean
}>()
const emit = defineEmits<{
  (e: 'installSuccess'): void
}>()
const { message } = useGlobalApp()

const router = useRouter()
const formRef = ref<FormInstance>()
const loading = ref(false)

const modelRef = reactive({
  email: '',
  username: '',
  password: '',
})

const rulesRef = reactive({
  email: [
    {
      required: true,
      type: 'email' as const,
      message: () => $gettext('Please input your E-mail!'),
    },
  ],
  username: [
    {
      required: true,
      message: () => $gettext('Please input your username!'),
    },
    {
      max: 255,
      message: () => $gettext('Username length cannot exceed 255 characters'),
    },
  ],
  password: [
    {
      required: true,
      message: () => $gettext('Please input your password!'),
    },
    {
      max: 20,
      message: () => $gettext('Password length cannot exceed 20 characters'),
    },
  ],
})

async function onSubmit() {
  await formRef.value?.validate()

  loading.value = true

  try {
    if (props.frontendDebug) {
      await new Promise(resolve => setTimeout(resolve, 300))
      message.success($gettext('Frontend debug mode: install flow completed without sending a backend request'))
      emit('installSuccess')
      return
    }

    await install.install_nginx_ui(modelRef, props.installSecret)
    message.success($gettext('Install successfully'))
    emit('installSuccess')
    await router.push('/login')
  }
  finally {
    loading.value = false
  }
}
</script>

<template>
  <AForm
    id="components-form-install"
    ref="formRef"
    :model="modelRef"
    :rules="rulesRef"
  >
    <AFormItem name="email">
      <AInput
        v-model:value="modelRef.email"
        :placeholder="$gettext('Email (*)')"
      >
        <template #prefix>
          <MailOutlined />
        </template>
      </AInput>
    </AFormItem>
    <AFormItem name="username">
      <AInput
        v-model:value="modelRef.username"
        :placeholder="$gettext('Username (*)')"
      >
        <template #prefix>
          <UserOutlined />
        </template>
      </AInput>
    </AFormItem>
    <AFormItem name="password">
      <AInputPassword
        v-model:value="modelRef.password"
        :placeholder="$gettext('Password (*)')"
      >
        <template #prefix>
          <LockOutlined />
        </template>
      </AInputPassword>
    </AFormItem>
    <AFormItem>
      <AButton
        type="primary"
        block
        html-type="submit"
        :loading="loading"
        @click="onSubmit"
      >
        {{ $gettext('Install') }}
      </AButton>
    </AFormItem>
  </AForm>
</template>
