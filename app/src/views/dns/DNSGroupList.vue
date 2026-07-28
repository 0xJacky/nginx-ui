<script setup lang="ts">
import type { TableColumnsType } from 'ant-design-vue'
import type { DNSDomain } from '@/api/dns'
import type { DNSDomainGroup } from '@/pinia/moudule/dnsGroup'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons-vue'
import { message } from 'ant-design-vue'
import dayjs from 'dayjs'
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useDnsGroupStore } from '@/pinia'
import { listAllDNSDomains } from '@/views/dns/group'

const router = useRouter()
const groupStore = useDnsGroupStore()
const domains = ref<DNSDomain[]>([])
const isLoading = ref(false)
const isDrawerOpen = ref(false)
const editingGroupId = ref<string | null>(null)

const formModel = reactive({
  name: '',
  description: '',
  domainIds: [] as number[],
})

const columns: TableColumnsType = [{
  title: $gettext('Group'),
  key: 'name',
}, {
  title: $gettext('Domains'),
  key: 'domains',
}, {
  title: $gettext('Updated at'),
  key: 'updatedAt',
  width: 180,
}, {
  title: $gettext('Actions'),
  key: 'actions',
  width: 260,
  fixed: 'right',
}]

const domainMap = computed(() => new Map(domains.value.map(domain => [domain.id, domain])))
const domainOptions = computed(() => domains.value.map(domain => ({
  label: `${domain.domain} · ${domain.dns_credential?.provider ?? $gettext('Unknown provider')}`,
  value: domain.id,
})))

const selectedDomainOptions = computed(() => {
  const options = [...domainOptions.value]
  for (const domainId of formModel.domainIds) {
    if (!domainMap.value.has(domainId)) {
      options.push({
        label: $gettext('Unavailable domain #%{id}', { id: String(domainId) }),
        value: domainId,
      })
    }
  }
  return options
})

async function loadDomains() {
  isLoading.value = true
  try {
    domains.value = await listAllDNSDomains()
  }
  finally {
    isLoading.value = false
  }
}

function resetForm() {
  editingGroupId.value = null
  formModel.name = ''
  formModel.description = ''
  formModel.domainIds = []
}

function openCreateDrawer() {
  resetForm()
  isDrawerOpen.value = true
}

function openEditDrawer(group: DNSDomainGroup) {
  editingGroupId.value = group.id
  formModel.name = group.name
  formModel.description = group.description
  formModel.domainIds = [...group.domainIds]
  isDrawerOpen.value = true
}

function closeDrawer() {
  isDrawerOpen.value = false
  resetForm()
}

function saveGroup() {
  const name = formModel.name.trim()
  if (!name) {
    message.warning($gettext('Enter a group name'))
    return
  }
  if (formModel.domainIds.length < 2) {
    message.warning($gettext('Select at least two domains'))
    return
  }

  const hasDuplicateName = groupStore.groups.some(group =>
    group.id !== editingGroupId.value
    && group.name.trim().toLowerCase() === name.toLowerCase(),
  )
  if (hasDuplicateName) {
    message.warning($gettext('A group with this name already exists'))
    return
  }

  const input = {
    name,
    description: formModel.description,
    domainIds: formModel.domainIds,
  }
  if (editingGroupId.value) {
    groupStore.updateGroup(editingGroupId.value, input)
    message.success($gettext('Group updated'))
  }
  else {
    groupStore.createGroup(input)
    message.success($gettext('Group created'))
  }
  closeDrawer()
}

function deleteGroup(group: DNSDomainGroup) {
  groupStore.deleteGroup(group.id)
  message.success($gettext('Group deleted'))
}

function manageGroupRecords(group: DNSDomainGroup) {
  router.push({
    name: 'DNS Group Records',
    params: { id: group.id },
  })
}

function getGroupDomains(group: DNSDomainGroup) {
  return group.domainIds.map(domainId => ({
    id: domainId,
    name: domainMap.value.get(domainId)?.domain ?? $gettext('Unavailable domain #%{id}', { id: String(domainId) }),
    isMissing: !domainMap.value.has(domainId),
  }))
}

function formatTime(value: string) {
  return dayjs(value).format('YYYY-MM-DD HH:mm')
}

onMounted(loadDomains)
</script>

<template>
  <div class="dns-groups-page">
    <AAlert
      type="info"
      show-icon
      class="mb-4"
      :message="$gettext('Groups are stored in this browser')"
      :description="$gettext('Groups are scoped to the current user and selected node. Record operations run once and are not continuous synchronization.')"
    />

    <ACard>
      <template #title>
        {{ $gettext('DNS Groups') }}
      </template>
      <template #extra>
        <ASpace>
          <AButton :loading="isLoading" @click="loadDomains">
            <template #icon>
              <ReloadOutlined />
            </template>
            {{ $gettext('Refresh') }}
          </AButton>
          <AButton type="primary" @click="openCreateDrawer">
            <template #icon>
              <PlusOutlined />
            </template>
            {{ $gettext('Create group') }}
          </AButton>
        </ASpace>
      </template>

      <ATable
        row-key="id"
        :columns="columns"
        :data-source="groupStore.groups"
        :loading="isLoading"
        :pagination="false"
        :scroll="{ x: 860 }"
      >
        <template #emptyText>
          <AEmpty :description="$gettext('Create a group to manage the same DNS record across multiple domains')">
            <AButton type="primary" @click="openCreateDrawer">
              {{ $gettext('Create group') }}
            </AButton>
          </AEmpty>
        </template>
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'name'">
            <div class="group-name">
              {{ (record as DNSDomainGroup).name }}
            </div>
            <div v-if="(record as DNSDomainGroup).description" class="group-description">
              {{ (record as DNSDomainGroup).description }}
            </div>
          </template>
          <template v-else-if="column.key === 'domains'">
            <ASpace wrap size="small">
              <ATag
                v-for="domain in getGroupDomains(record as DNSDomainGroup)"
                :key="domain.id"
                :color="domain.isMissing ? 'error' : undefined"
              >
                {{ domain.name }}
              </ATag>
            </ASpace>
          </template>
          <template v-else-if="column.key === 'updatedAt'">
            {{ formatTime((record as DNSDomainGroup).updatedAt) }}
          </template>
          <template v-else-if="column.key === 'actions'">
            <ASpace size="small">
              <AButton type="link" size="small" @click="manageGroupRecords(record as DNSDomainGroup)">
                {{ $gettext('Manage records') }}
              </AButton>
              <AButton type="link" size="small" @click="openEditDrawer(record as DNSDomainGroup)">
                {{ $gettext('Edit group') }}
              </AButton>
              <APopconfirm
                :title="$gettext('Delete this local group? DNS records will not be changed.')"
                :ok-text="$gettext('Delete group')"
                :cancel-text="$gettext('Cancel')"
                @confirm="deleteGroup(record as DNSDomainGroup)"
              >
                <AButton type="link" size="small" danger>
                  {{ $gettext('Delete') }}
                </AButton>
              </APopconfirm>
            </ASpace>
          </template>
        </template>
      </ATable>
    </ACard>

    <ADrawer
      :open="isDrawerOpen"
      :title="editingGroupId ? $gettext('Edit DNS group') : $gettext('Create DNS group')"
      width="520"
      @close="closeDrawer"
    >
      <AForm layout="vertical">
        <AFormItem :label="$gettext('Group name')" required>
          <AInput v-model:value="formModel.name" :maxlength="80" show-count />
        </AFormItem>
        <AFormItem :label="$gettext('Description')">
          <ATextarea v-model:value="formModel.description" :auto-size="{ minRows: 2, maxRows: 5 }" :maxlength="240" show-count />
        </AFormItem>
        <AFormItem :label="$gettext('Domains')" required>
          <ASelect
            v-model:value="formModel.domainIds"
            mode="multiple"
            show-search
            option-filter-prop="label"
            max-tag-count="responsive"
            :options="selectedDomainOptions"
            :loading="isLoading"
            :placeholder="$gettext('Select at least two domains')"
          />
          <div class="form-help">
            {{ $gettext('Batch record operations target the selected domains. Provider credentials remain unchanged.') }}
          </div>
        </AFormItem>
      </AForm>
      <template #footer>
        <ASpace>
          <AButton @click="closeDrawer">
            {{ $gettext('Cancel') }}
          </AButton>
          <AButton type="primary" @click="saveGroup">
            {{ editingGroupId ? $gettext('Save group') : $gettext('Create group') }}
          </AButton>
        </ASpace>
      </template>
    </ADrawer>
  </div>
</template>

<style scoped>
.dns-groups-page {
  padding-bottom: 16px;
}

.group-name {
  font-weight: 600;
}

.group-description,
.form-help {
  margin-top: 4px;
  color: var(--ant-color-text-secondary);
  font-size: 12px;
  line-height: 1.5;
}
</style>
