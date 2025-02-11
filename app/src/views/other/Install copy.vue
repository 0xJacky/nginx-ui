<script setup lang="ts">
import install from '@/api/install'
import SetLanguage from '@/components/SetLanguage/SetLanguage.vue'
import SwitchAppearance from '@/components/SwitchAppearance/SwitchAppearance.vue'
import { DatabaseOutlined, LockOutlined, MailOutlined, UserOutlined } from '@ant-design/icons-vue'

import { Form, message } from 'ant-design-vue'
import { useRouter } from 'vue-router'

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
    {
      max: 20,
      message: () => $gettext('Password length cannot exceed 20 characters'),
    },
  ],
  database: [
    {
      message: () =>
        $gettext('The filename cannot contain the following characters: %{c}', { c: '& &quot; ? < > # {} % ~ / \\' }),
    },
  ],
})

const { validate, validateInfos } = Form.useForm(modelRef, rulesRef)

function onSubmit() {
  validate().then(() => {
    // modelRef
    loading.value = true

    install.install_nginx_ui(modelRef).then(async () => {
      message.success($gettext('Install successfully'))
      await router.push('/login')
    }).finally(() => {
      loading.value = false
    })
  })
}
</script>
<template>
  <ALayout>
    <ALayoutContent>
      <div class="install-container">
        <ACard class="install-card" :bordered="false">
          <div class="install-form">
            <div class="project-title">
              <h1>PrimeWaf</h1>
            </div>
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
            <div class="footer">
              <p>Copyright © 2021 - {{ thisYear }} PrimeWaf</p>
              Language
              <SetLanguage class="inline" />
              <div class="flex justify-center mt-4">
                <SwitchAppearance />
              </div>
            </div>
          </div>
        </ACard>
      </div>
    </ALayoutContent>
  </ALayout>
</template>

<style lang="less" scoped>
.install-container {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100vh;
  background: linear-gradient(135deg, #f5f7fa 0%, #c3cfe2 100%);

  .install-card {
    max-width: 420px;
    width: 90%;
    border-radius: 15px;
    box-shadow: 0 8px 30px rgba(0, 0, 0, 0.1);
    backdrop-filter: blur(10px);
    background: rgba(255, 255, 255, 0.95);
    
    .install-form {
      .project-title {
        margin: 30px 0;
        
        h1 {
          font-size: 42px;
          font-weight: 300;
          text-align: center;
          background: linear-gradient(45deg, #2196F3, #00BCD4);
          -webkit-background-clip: text;
          -webkit-text-fill-color: transparent;
          letter-spacing: 1px;
        }
      }

      :deep(.ant-input-affix-wrapper) {
        border-radius: 8px;
        height: 45px;
      }

      :deep(.ant-btn) {
        height: 45px;
        border-radius: 8px;
        font-weight: 500;
        transition: all 0.3s ease;
        
        &:hover {
          transform: translateY(-2px);
          box-shadow: 0 5px 15px rgba(0, 0, 0, 0.1);
        }
      }

      .anticon {
        color: #2196F3 !important;
      }

      .footer {
        padding: 20px;
        text-align: center;
        font-size: 14px;
        color: #666;
      }
    }
  }
}

.dark {
  .install-container {
    background: linear-gradient(135deg, #1a1a1a 0%, #2d3436 100%);
    
    .install-card {
      background: rgba(30, 30, 30, 0.95);
    }
  }
}
</style>
