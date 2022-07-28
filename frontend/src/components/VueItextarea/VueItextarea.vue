<template>
    <editor v-model="current_value" @init="editorInit" lang="nginx" theme="monokai" width="100%" :height="defaultTextHeight"></editor>
</template>
<style lang="less">
.cm-s-monokai {
    height: auto !important;
}
</style>
<script>
import 'codemirror/lib/codemirror.css'
import 'codemirror/theme/monokai.css'

import 'codemirror/mode/nginx/nginx'

export default {
    name: 'vue-itextarea',
    components: {
       // codemirror
        editor: require('vue2-ace-editor'),
    },
    props: {
        value: {},
        defaultTextHeight: {
            type: Number,
            default: 1000
        }
    },
    model: {
        prop: 'value',
        event: 'changeValue'
    },
    watch: {
        value() {
            this.current_value = this.value ?? ''
        },
        current_value() {
            this.$emit('changeValue', this.current_value)
        }
    },
    data() {
        return {
            current_value: this.value ?? '',
        }
    },
    methods: {
        editorInit: function () {
            require('brace/ext/language_tools') //language extension prerequsite...
            require('brace/mode/nginx')    //language
            require('brace/theme/monokai')
        }
    }
}
</script>
