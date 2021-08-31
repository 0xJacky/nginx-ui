<template>
    <div class="login-form">
        <div class="project-title">
            <h1>Nginx UI</h1>
        </div>
        <a-form
            id="components-form-demo-normal-login"
            :form="form"
            class="login-form"
            @submit="handleSubmit"
        >
            <a-form-item>
                <a-input
                    v-decorator="[
          'email',
          { rules: [{
                type: 'email',
                message: 'The input is not valid E-mail!',
              },
              {
                required: true,
                message: 'Please input your E-mail!',
              },] },
        ]"
                    placeholder="Email"
                >
                    <a-icon slot="prefix" type="mail" style="color: rgba(0,0,0,.25)"/>
                </a-input>
            </a-form-item>
            <a-form-item>
                <a-input
                    v-decorator="[
          'username',
          { rules: [{ required: true, message: 'Please input your username!' }] },
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
          { rules: [{ required: true, message: 'Please input your Password!' }] },
        ]"
                    type="password"
                    placeholder="Password"
                >
                    <a-icon slot="prefix" type="lock" style="color: rgba(0,0,0,.25)"/>
                </a-input>
            </a-form-item>
            <a-form-item>
                <a-button type="primary" :block="true" html-type="submit" :loading="loading">
                    安装
                </a-button>
            </a-form-item>
        </a-form>
        <footer>
            Copyright © 2020 - {{ thisYear }} 0xJacky
        </footer>
    </div>

</template>

<script>
export default {
    name: 'Login',
    data() {
        return {
            form: {},
            lock: true,
            thisYear: new Date().getFullYear(),
            loading: false
        }
    },
    created() {
        this.form = this.$form.createForm(this)
    },
    mounted() {
        this.$api.install.get_lock().then(r => {
            if (r.lock) {
                this.$router.push('/login')
            }
        })
    },
    methods: {
        handleSubmit: async function (e) {
            e.preventDefault()
            this.loading = true
            await this.form.validateFields(async (err, values) => {
                if (!err) {
                    this.$api.install.install_nginx_ui(values).then(() => {
                        this.$router.push('/login')
                    })
                }
                this.loading = false
            })
        },
    },
};
</script>
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
