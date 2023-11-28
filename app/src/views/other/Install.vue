<script setup lang="ts">
import { Form, message } from 'ant-design-vue'
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { DatabaseOutlined, LockOutlined, MailOutlined, UserOutlined } from '@ant-design/icons-vue'
import SetLanguage from '@/components/SetLanguage/SetLanguage.vue'
import gettext from '@/gettext'
import install from '@/api/install'

const { $gettext, interpolate } = gettext

const thisYear = new Date().getFullYear()
const loading = ref(false)

const router = useRouter()

install.get_lock().then(async (r: { lock: boolean }) => {
  if (r.lock)
    await router.push('/login')
})

const modelRef = reactive({
  email: '',
  username: '',
  password: '',
  database: '',
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
  ],
  password: [
    {
      required: true,
      message: () => $gettext('Please input your password!'),
    },
  ],
  database: [
    {
      message: () => interpolate(
        $gettext('The filename cannot contain the following characters: %{c}'),
        { c: '& &quot; ? < > # {} % ~ / \\' },
      ),
    },
  ],
})

const { validate, validateInfos } = Form.useForm(modelRef, rulesRef)

const onSubmit = () => {
  validate().then(() => {
    // modelRef
    loading.value = true
    // eslint-disable-next-line promise/no-nesting
    install.install_nginx_ui(modelRef).then(async () => {
      message.success($gettext('Install successfully'))
      await router.push('/login')
      // eslint-disable-next-line promise/no-nesting
    }).catch(e => {
      message.error(e.message ?? $gettext('Server error'))
    }).finally(() => {
      loading.value = false
    })
  })
}
</script>

<template>
  <div class="login-form">
    <div class="project-title">
      <h1>Nginx UI</h1>
    </div>
    <AForm
      id="components-form-install"
      class="login-form"
    >
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
        <AInput
          v-bind="validateInfos.database"
          v-model:value="modelRef.database"
          :placeholder="$gettext('Database (Optional, default: database)')"
        >
          <template #prefix>
            <DatabaseOutlined />
          </template>
        </AInput>
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
    <footer>
      Copyright © 2020 - {{ thisYear }} Nginx UI | Language
      <SetLanguage
        class="set_lang"
        style="display: inline"
      />
    </footer>
  </div>
</template>

<style lang="less">
.project-title {
  margin: 50px;

  h1 {
    font-size: 50px;
    font-weight: 100;
    text-align: center;
  }
}

.login-form {
  max-width: 500px;
  margin: 0 auto;
}

.login-form-button {

}

footer {
  padding: 30px;
  text-align: center;
}
</style>
