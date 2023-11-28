<script setup lang="ts">
import StdCurd from '@/components/StdDesign/StdDataDisplay/StdCurd.vue'
import gettext from '@/gettext'
import user from '@/api/user'
import { datetime } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { input, password } from '@/components/StdDesign/StdDataEntry'

const { $gettext } = gettext

const columns = [{
  title: () => $gettext('Username'),
  dataIndex: 'name',
  sorter: true,
  pithy: true,
  edit: {
    type: input,
  },
  search: true,
}, {
  title: () => $gettext('Password'),
  dataIndex: 'password',
  sorter: true,
  pithy: true,
  edit: {
    type: password,
    placeholder: () => $gettext('Leave blank for no change'),
    generate: true,
  },
  display: false,
}, {
  title: () => $gettext('Created at'),
  dataIndex: 'created_at',
  customRender: datetime,
  sorter: true,
  pithy: true,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'updated_at',
  customRender: datetime,
  sorter: true,
  pithy: true,
}, {
  title: () => $gettext('Action'),
  dataIndex: 'action',
}]
</script>

<template>
  <StdCurd
    :title="$gettext('Manage Users')"
    :columns="columns"
    :api="user"
  />
</template>

<style scoped>

</style>
