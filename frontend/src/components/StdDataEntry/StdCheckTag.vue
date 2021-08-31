<template>
    <div>
        <template v-for="(v,k) in options">
            <a-checkable-tag
                :key="k"
                :checked="selectedTag === k"
                @change="() => handleChange(k)"
            >
                {{ v }}
            </a-checkable-tag>
        </template>
    </div>
</template>

<script>
export default {
    name: "StdCheckTag",
    data() {
        return {
            selectedTag: '',
        }
    },
    props: {
        value: [Number, String, Boolean],
        options: [Array, Object],
        keyType: {
            type: String,
            default() {
                return 'int'
            }
        }
    },
    model: {
        prop: 'value',
        event: 'change'
    },
    methods: {
        handleChange(tag) {
            this.selectedTag = tag
            this.$emit('change', isNaN(parseInt(tag)) || this.keyType === 'string' ? tag : parseInt(tag))
        }
    },
    watch: {
        value() {
            this.selectedTag = this.value != null ? this.value.toString() : null
        }
    },
    created() {
        this.selectedTag = this.value != null ? this.value.toString() : null
    }
}
</script>

<style lang="less" scoped>
.ant-tag {
    background-color: rgba(0, 0, 0, 0.05);
}
.ant-tag-checkable-checked {
    background-color: #1890ff;
}
</style>
