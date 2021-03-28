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
          'name',
          { rules: [{ required: true, message: 'Please input your username!' }] },
        ]"
                    placeholder="Username"
                >
                    <a-icon slot="prefix" type="user" style="color: rgba(0,0,0,.25)" />
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
                    <a-icon slot="prefix" type="lock" style="color: rgba(0,0,0,.25)" />
                </a-input>
            </a-form-item>
            <a-form-item>
                <a-button type="primary" :block="true" html-type="submit">
                    Log in
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
            thisYear: new Date().getFullYear()
        }
    },
    created() {
        if (this.$store.state.user.token) {
            this.$router.push('/')
        }
        this.form = this.$form.createForm(this)
    },
    methods: {
        handleSubmit(e) {
            e.preventDefault()
            this.form.validateFields((err, values) => {
                if (!err) {
                    this.$api.auth.login(values.name, values.password).then(async () => {
                        await this.$message.success('登录成功', 1)
                        const next = this.$route.query.next ? this.$route.query.next : '/'
                        await this.$router.push(next)
                    }).catch(r => {
                        console.log(r)
                        this.$message.error(r.message)
                    })
                }
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
