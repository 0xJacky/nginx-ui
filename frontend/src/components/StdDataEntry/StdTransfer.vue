<template>
    <a-transfer
        :data-source="dataSource"
        :render="item=>item.title"
        :selectedKeys="selectedKeys"
        :targetKeys="targetKeys"
        :titles="['可添加', '已添加']"
        @change="handleChange"
        @selectChange="handleSelectChange"
    />
</template>

<script>
const mockData = []
for (let i = 0; i < 20; i++) {
    mockData.push({
        key: i.toString(),
        title: `content${i + 1}`,
        description: `description of content${i + 1}`
    })
}
export default {
    name: 'StdTransfer',
    props: {
        api: Function,
        dataKey: String,
        target: String
    },
    model: {
        prop: 'target',
        event: 'changeTarget'
    },
    data() {
        return {
            targetKeys: [],
            selectedKeys: [],
            dataSource: []
        }
    },
    created() {
        this.targetKeys = this.target.split(',')
        if (this.api) {
            this.api().then(r => {
                const dataSource = []
                r[this.dataKey ? this.dataKey : 'data'].forEach(v => {
                    dataSource.push({
                        key: v.id.toString(),
                        title: `${v.title}`,
                        description: `${v.description}`
                    })
                })
                this.dataSource = dataSource
            })
        }
    },
    watch: {
        targetKeys() {
            this.$emit('changeTarget', this.targetKeys.toString())
        }
    },
    methods: {
        handleChange(nextTargetKeys) {
            this.targetKeys = nextTargetKeys
        },
        handleSelectChange(sourceSelectedKeys, targetSelectedKeys) {
            this.selectedKeys = [...sourceSelectedKeys, ...targetSelectedKeys]

        },
    }
}
</script>

<style scoped>

</style>
