<template>
    <div class="container">
        <div class="login-form">
            <div class="project-title">
                <h1>Nginx UI</h1>
            </div>
            <a-form
                id="components-form-demo-normal-login"
                :form="form"
                @submit="handleSubmit"
            >
                <a-form-item>
                    <a-input
                        v-decorator="[
          'name',
          { rules: [{ required: true, message: '请输入用户名' }] },
        ]"
                        placeholder="Username"
                    >
                        <a-icon slot="prefix" type="user" style="color: rgba(0,0,0,.25)"/>
                    </a-input>
                </a-form-item>
                <a-form-item>
                    <a-input
                        v-decorator="[
          'password',
          { rules: [{ required: true, message: '请输入密码' }] },
        ]"
                        type="password"
                        placeholder="Password"
                    >
                        <a-icon slot="prefix" type="lock" style="color: rgba(0,0,0,.25)"/>
                    </a-input>
                </a-form-item>
                <a-form-item>
                    <a-button type="primary" :block="true" html-type="submit" :loading="loading">
                        登录
                    </a-button>
                </a-form-item>
            </a-form>
            <div class="footer">
                Copyright © 2020 - {{ thisYear }} Nginx UI
            </div>
        </div>
    </div>
</template>

<script>
export default {
    name: 'Login',
    data() {
        return {
            form: {},
            thisYear: new Date().getFullYear(),
            loading: false
        }
    },
    created() {
        this.form = this.$form.createForm(this)
    },
    mounted() {
        this.$api.install.get_lock().then(r => {
            if (!r.lock) {
                this.$router.push('/install')
            }
        })
        if (this.$store.state.user.token) {
            this.$router.push('/')
        }
    },
    methods: {
        login(values) {
            return this.$api.auth.login(values.name, values.password).then(async () => {
                await this.$message.success('登录成功', 1)
                const next = this.$route.query.next ? this.$route.query.next : '/'
                await this.$router.push(next)
            }).catch(r => {
                console.log(r)
                this.$message.error(r.message ?? '服务器错误')
            })
        },
        handleSubmit: async function (e) {
            e.preventDefault()
            this.loading = true
            await this.form.validateFields(async (err, values) => {
                if (!err) {
                    await this.login(values)
                }
                this.loading = false
            })
        },
    },
}
</script>
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

        .login-form-button {

        }

        .footer {
            padding: 30px;
            text-align: center;
        }
    }
}

</style>
