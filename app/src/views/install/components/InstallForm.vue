<script setup lang="ts">
import { LockOutlined, MailOutlined, UserOutlined } from '@ant-design/icons-vue'
import { Form, message } from 'ant-design-vue'
import { useRouter } from 'vue-router'
import install from '@/api/install'

const emit = defineEmits<{
  (e: 'installSuccess'): void
}>()

const router = useRouter()
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
      type: 'email',
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

const { validate, validateInfos } = Form.useForm(modelRef, rulesRef)

function onSubmit() {
  validate().then(() => {
    loading.value = true

    install.install_nginx_ui(modelRef).then(async () => {
      message.success($gettext('Install successfully'))
      emit('installSuccess')
      await router.push('/login')
    }).catch(error => {
      if (error && error.code === 40308)
        throw error
    }).finally(() => {
      loading.value = false
    })
  })
}
</script>

<template>
  <AForm id="components-form-install">
    <AFormItem v-bind="validateInfos.email">
      <AInput
        v-model:value="modelRef.email"
        :placeholder="$gettext('Email (*)')"
      >
        <template #prefix>
          <MailOutlined />
        </template>
      </AInput>
    </AFormItem>
    <AFormItem v-bind="validateInfos.username">
      <AInput
        v-model:value="modelRef.username"
        :placeholder="$gettext('Username (*)')"
      >
        <template #prefix>
          <UserOutlined />
        </template>
      </AInput>
    </AFormItem>
    <AFormItem v-bind="validateInfos.password">
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
