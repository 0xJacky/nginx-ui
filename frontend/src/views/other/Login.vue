<script setup lang="ts">
import {useUserStore} from '@/pinia'
import {LockOutlined, UserOutlined} from '@ant-design/icons-vue'
import {reactive, ref, watch} from 'vue'
import {useRoute, useRouter} from 'vue-router'
import gettext from '@/gettext'
import {Form, message} from 'ant-design-vue'
import auth from '@/api/auth'
import install from '@/api/install'
import SetLanguage from '@/components/SetLanguage/SetLanguage.vue'

const thisYear = new Date().getFullYear()

const route = useRoute()
const router = useRouter()

install.get_lock().then(async (r: { lock: boolean }) => {
    if (!r.lock) {
        await router.push('/install')
    }
})

const {$gettext} = gettext
const loading = ref(false)

const modelRef = reactive({
    username: '',
    password: ''
})

const rulesRef = reactive({
    username: [
        {
            required: true,
            message: () => $gettext('Please input your username!')
        }
    ],
    password: [
        {
            required: true,
            message: () => $gettext('Please input your password!')
        }
    ]
})

const {validate, validateInfos, clearValidate} = Form.useForm(modelRef, rulesRef)

const onSubmit = () => {
    validate().then(async () => {
        loading.value = true
        await auth.login(modelRef.username, modelRef.password).then(async () => {
            message.success($gettext('Login successful'), 1)
            const next = (route.query?.next || '').toString() || '/'
            await router.push(next)
        }).catch(e => {
            message.error($gettext(e.message ?? 'Server error'))
        })
        loading.value = false
    })
}

const user = useUserStore()

if (user.is_login) {
    const next = (route.query?.next || '').toString() || '/dashboard'
    router.push(next)
}

watch(() => gettext.current, () => {
    clearValidate()
})

</script>

<template>
    <div class="container">
        <div class="login-form">
            <div class="project-title">
                <h1>Nginx UI</h1>
            </div>
            <a-form id="components-form-demo-normal-login">
                <a-form-item v-bind="validateInfos.username">
                    <a-input
                        v-model:value="modelRef.username"
                        :placeholder="$gettext('Username')"
                    >
                        <template #prefix>
                            <UserOutlined style="color: rgba(0, 0, 0, 0.25)"/>
                        </template>
                    </a-input>
                </a-form-item>
                <a-form-item v-bind="validateInfos.password">
                    <a-input-password
                        v-model:value="modelRef.password"
                        :placeholder="$gettext('Password')"
                    >
                        <template #prefix>
                            <LockOutlined style="color: rgba(0, 0, 0, 0.25)"/>
                        </template>
                    </a-input-password>
                </a-form-item>
                <a-form-item>
                    <a-button @click="onSubmit" type="primary" :block="true" html-type="submit" :loading="loading">
                        {{ $gettext('Login') }}
                    </a-button>
                </a-form-item>
            </a-form>
            <div class="footer">
                <p>Copyright © 2020 - {{ thisYear }} Nginx UI</p>
                Language
                <set-language class="set_lang" style="display: inline"/>
            </div>
        </div>
    </div>
</template>

<style lang="less">
.container {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;

    .login-form {
        max-width: 400px;
        width: 80%;

        .project-title {
            margin: 50px;

            h1 {
                font-size: 50px;
                font-weight: 100;
                text-align: center;
            }
        }

        .anticon {
            color: #a8a5a5 !important;
        }

        .login-form-button {

        }

        .footer {
            padding: 30px;
            text-align: center;
        }
    }
}

</style>
