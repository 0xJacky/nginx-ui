<template>
    <div>
        <template v-for="(v,k) in options">
            <a-checkable-tag
                :key="k"
                :checked="selectedTags.indexOf(k) > -1"
                @change="checked => handleChange(k, checked)"
            >
                {{ v }}
            </a-checkable-tag>
        </template>
    </div>
</template>

<script>
export default {
    name: 'StdMultiCheckTag',
    data() {
        return {
            selectedTags: [],
        }
    },
    props: {
        disabled: [Boolean],
        value: [Array],
        dataObject: [Object],
        options: {
            type: Object,
            default() {
                return {}
            }
        },
    },
    model: {
        prop: 'value',
        event: 'change'
    },
    methods: {
        handleChange(tag, checked) {
            const {selectedTags} = this
            this.selectedTags = checked
                ? [...selectedTags, tag]
                : selectedTags.filter(t => t !== tag)
            this.$emit('change', this.selectedTags)
        },
        loadData() {
            for (const [k] of Object.entries(this.options)) {
                if (this.dataObject[k] === 1) {
                    if (this.selectedTags.indexOf(k) === -1)
                        this.selectedTags.push(k)
                }
            }
        }
    },
    watch: {
        value() {
            this.selectedTag = this.value ?? []
        },
    },
    created() {
        this.selectedTag = this.value ?? []
        this.loadData()
    },
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
