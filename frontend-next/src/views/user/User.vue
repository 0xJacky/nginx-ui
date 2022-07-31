<script setup lang="ts">
import StdCurd from '@/components/StdDataDisplay/StdCurd.vue'
import gettext from '@/gettext'
import user from '@/api/user'
import {datetime} from '@/components/StdDataDisplay/StdTableTransformer'

const {$gettext} = gettext

const columns = [{
    title: $gettext('Username'),
    dataIndex: 'name',
    sorter: true,
    pithy: true,
    edit: {
        type: 'input'
    }
}, {
    title: $gettext('Password'),
    dataIndex: 'password',
    sorter: true,
    pithy: true,
    edit: {
        type: 'input',
        placeholder: $gettext('Leave blank for no change')
    },
    display: false
}, {
    title: $gettext('Created at'),
    dataIndex: 'created_at',
    customRender: datetime,
    sorter: true,
    pithy: true
}, {
    title: $gettext('Updated at'),
    dataIndex: 'updated_at',
    customRender: datetime,
    sorter: true,
    pithy: true
}, {
    title: $gettext('Action'),
    dataIndex: 'action'
}]
</script>

<template>
    <std-curd :columns="columns" :api="user" :disable_search="true"/>
</template>


<style scoped>

</style>
