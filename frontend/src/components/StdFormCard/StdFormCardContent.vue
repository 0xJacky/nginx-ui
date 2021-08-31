<template>
    <a-card :bordered="false" class="form-card">
        <a-form :form="form" @submit="handleSubmit">
            <div class="logo">
                <img :src="logo"/>
            </div>
            <p class="title">
                {{ options.title }}
            </p>
            <a-form-item
                v-for="item in options.items"
                :key="item.label"
                :help="errors[item.decorator[0]] ? errors[item.decorator[0]] : null"
                :label="item.label"
                :validate-status="errors[item.decorator[0]] ? 'error' :'success'"
            >
                <a-input
                    v-decorator="item.decorator"
                    :autocomplate="item.autocomplate ? 'on' : 'off'"
                    :placeholder="item.placeholder"
                    :type="item.type"
                >
                    <a-icon slot="prefix" :type="item.icon" style="color: rgba(0,0,0,.25)"/>
                </a-input>
            </a-form-item>
            <div class="action">
                <div class="center">
                    <a-button
                        :loading="loading"
                        class="std-border-radius"
                        html-type="submit"
                        type="primary"
                    >
                        {{ options.button_text }}
                    </a-button>
                </div>
                <div class="small-link center">
                    <slot name="small-link"/>
                </div>
            </div>
        </a-form>
    </a-card>
</template>

<script>
//import {VueReCaptcha} from 'vue-recaptcha-v3'
//import Vue from 'vue'

/*Vue.use(VueReCaptcha, {
    siteKey: process.env.VUE_APP_RECAPTCHA_SITEKEY,
    loaderOptions: {
        useRecaptchaNet: true
    }
})*/

export default {
    name: 'StdFormCardContent',
    props: {
        options: Object,
        errors: {
            type: Object,
            default() {
                return {}
            }
        },
    },
    data() {
        return {
            logo: require('@/assets/img/logo.png'),
            loading: false,
            form: null
        }
    },
    mounted() {
        this.form = this.$form.createForm(this)
    },
    methods: {
        async handleSubmit(e) {
            e.preventDefault()
            this.form.validateFields((err, values) => {
                if (!err) {
                    this.loading = true
                    //this.$recaptchaLoaded().then(() => {
                    //this.$recaptcha('std_form').then(token => {
                    //values.token = token
                    this.$emit('onSubmit', values)
                    //})
                    // })
                    this.loading = false
                }
            })
        }
    }
}
</script>

<style lang="less">
.form-card {
    .ant-form-item {
        input {
            border-radius: 20px;
        }
    }
}
</style>

<style lang="less" scoped>
.form-card {
    box-shadow: 0 0 30px rgba(200, 200, 200, 0.25);

    .ant-form {
        max-width: 250px;
        margin: 0 auto;

        .title {
            text-align: center;
            font-size: 17px;
            margin: 10px 0;
        }
    }

    .logo {
        display: grid;
        padding: 10px;

        img {
            height: 80px;
            margin: 0 auto;
        }
    }

    .action {
        .small-link {
            font-size: 13px;
            padding: 15px 0 0 0;
        }
    }
}
</style>
