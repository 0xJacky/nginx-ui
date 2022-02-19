<template>
    <codemirror v-model="current_value" :options="cmOptions"/>
</template>
<style lang="less">
.cm-s-monokai {
    height: auto !important;
}
</style>
<script>
import {codemirror} from 'vue-codemirror'
import 'codemirror/lib/codemirror.css'
import 'codemirror/theme/monokai.css'

import 'codemirror/mode/nginx/nginx'

export default {
    name: 'vue-itextarea',
    components: {
        codemirror
    },
    props: {
        value: {},
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
            cmOptions: {
                tabSize: 4,
                mode: 'text/x-nginx-conf',
                theme: 'monokai',
                lineNumbers: true,
                line: true,
                highlightDifferences: true,
                defaultTextHeight: 1000,
                // more CodeMirror options...
            }
        }
    },
}
</script>
