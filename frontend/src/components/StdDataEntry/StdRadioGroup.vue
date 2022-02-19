<template>
    <a-radio-group name="radioGroup" v-model="data" @change="onChange">
        <a-radio :value="k" v-for="(v,k) in options" :key="k">
            {{ v }}
        </a-radio>
    </a-radio-group>
</template>


<script>
export default {
    name: 'StdRadioGroup',
    props: {
        options: [Object, Array],
        value: {
            type: [String, Number]
        },
        keyType: String
    },
    model: {
        prop: 'value',
        event: 'changeValue'
    },
    data() {
        return {
            data: this.value?.toString() ?? '',
        }
    },
    watch: {
        value() {
            this.data = this.value.toString()
        }
    },
    methods: {
        onChange(e) {
            if (this.keyType === 'int') {
                this.data = e.target.value
                this.$emit('changeValue', parseInt(e.target.value))
            } else {
                this.$emit('changeValue', e.target.value)
            }
        }
    }
}
</script>

<style scoped>

</style>
