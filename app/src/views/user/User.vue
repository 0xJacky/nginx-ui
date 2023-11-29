<script setup lang="ts">
import StdCurd from '@/components/StdDesign/StdDataDisplay/StdCurd.vue'
import gettext from '@/gettext'
import user from '@/api/user'
import { datetime } from '@/components/StdDesign/StdDataDisplay/StdTableTransformer'
import { input, password } from '@/components/StdDesign/StdDataEntry'
import type { Column } from '@/components/StdDesign/types'

const { $gettext } = gettext

const columns: Column[] = [{
  title: () => $gettext('Username'),
  dataIndex: 'name',
  sortable: true,
  pithy: true,
  edit: {
    type: input,
  },
  search: true,
}, {
  title: () => $gettext('Password'),
  dataIndex: 'password',
  sortable: true,
  pithy: true,
  edit: {
    type: password,
    config: {
      placeholder: () => $gettext('Leave blank for no change'),
      generate: true,
    },
  },
  hidden: true,
}, {
  title: () => $gettext('Created at'),
  dataIndex: 'created_at',
  customRender: datetime,
  sortable: true,
  pithy: true,
}, {
  title: () => $gettext('Updated at'),
  dataIndex: 'updated_at',
  customRender: datetime,
  sortable: true,
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
