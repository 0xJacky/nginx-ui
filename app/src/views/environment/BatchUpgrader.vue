<script setup lang="ts">
import _ from 'lodash'
import { message } from 'ant-design-vue'
import type { Ref } from 'vue'
import { marked } from 'marked'
import { useRoute } from 'vue-router'
import type { Environment } from '@/api/environment'
import upgrade, { type RuntimeInfo } from '@/api/upgrade'
import websocket from '@/lib/websocket'

const route = useRoute()
const visible = ref(false)
const nodeIds = ref<number[]>([])
const nodes = ref<Environment[]>([])
const channel = ref('stable')
const nodeNames = computed(() => nodes.value.map(v => v.name).join(', '))
const loading = ref(false)

const data = ref({
  name: '',
}) as Ref<RuntimeInfo>

const modalVisible = ref(false)
const modalClosable = ref(false)
const getReleaseError = ref(false)
const progressPercent = ref(0)
const progressStatus = ref('active') as Ref<'normal' | 'active' | 'success' | 'exception'>
const showLogContainer = ref(false)

const progressStrokeColor = {
  from: '#108ee9',
  to: '#87d068',
}

const logContainer = ref()
function log(msg: string) {
  const para = document.createElement('p')

  para.appendChild(document.createTextNode($gettext(msg)))

  logContainer.value.appendChild(para)

  logContainer.value.scroll({ top: 320, left: 0, behavior: 'smooth' })
}

const progressPercentComputed = computed(() => {
  return Number.parseFloat(progressPercent.value.toFixed(1))
})

function getLatestRelease() {
  loading.value = true
  data.value.body = ''
  upgrade.get_latest_release(channel.value).then(r => {
    data.value = r
  }).catch(e => {
    getReleaseError.value = e?.message
    message.error(e?.message ?? $gettext('Server error'))
  }).finally(() => {
    loading.value = false
  })
}

function open(selectedNodeIds: Ref<number[]>, selectedNodes: Ref<Environment[]>) {
  showLogContainer.value = false
  visible.value = true
  nodeIds.value = selectedNodeIds.value
  nodes.value = _.cloneDeep(selectedNodes.value)
  getLatestRelease()
}

watch(channel, getLatestRelease)

defineExpose({
  open,
})

const dryRun = computed(() => {
  return !!route.query.dry_run
})

// eslint-disable-next-line sonarjs/cognitive-complexity
async function performUpgrade() {
  showLogContainer.value = true
  progressStatus.value = 'active'
  modalClosable.value = false
  modalVisible.value = true
  progressPercent.value = 0
  logContainer.value.innerHTML = ''

  log($gettext('Upgrading Nginx UI, please wait...'))

  const nodesNum = nodes.value.length

  for (let i = 0; i < nodesNum; i++) {
    await new Promise(resolve => {
      const ws = websocket(`/api/upgrade/perform?x_node_id=${nodeIds.value[i]}`, false)

      let last = 0

      ws.onopen = () => {
        ws.send(JSON.stringify({
          dry_run: dryRun.value,
          channel: channel.value,
        }))
      }
      let isFailed = false

      ws.onmessage = async m => {
        const r = JSON.parse(m.data)
        if (r.message)
          log(r.message)
        switch (r.status) {
          case 'info':
            progressPercent.value += (10 / nodesNum)
            break
          case 'progress':
            progressPercent.value += ((r.progress - last) / 2) / nodesNum
            last = r.progress
            break
          case 'error':
            log('Upgraded successfully')
            isFailed = true
            break
          default:
            modalClosable.value = true
            break
        }
      }

      ws.onerror = () => {
        resolve({})
      }

      ws.onclose = async () => {
        resolve({})

        progressPercent.value = 100 * ((i + 1) / nodesNum)
        if (!isFailed)
          log($gettext('Upgraded Nginx UI on %{node} successfully 🎉', { node: nodes.value[i].name }))

        if (i + 1 === nodesNum) {
          progressStatus.value = 'success'
          modalClosable.value = true
        }
      }
    })
  }
}
</script>

<template>
  <AModal
    v-model:open="visible"
    :title="$gettext('Batch Upgrade')"
    :footer="false"
    :mask="false"
    width="800px"
  >
    <AForm layout="vertical">
      <AFormItem
        :label="$gettext('Channel')"
        class="max-w-40"
      >
        <ASelect v-model:value="channel">
          <ASelectOption key="stable">
            {{ $gettext('Stable') }}
          </ASelectOption>
          <ASelectOption key="prerelease">
            {{ $gettext('Pre-release') }}
          </ASelectOption>
        </ASelect>
      </AFormItem>
    </AForm>

    <ASpin :spinning="loading">
      <AAlert
        v-if="getReleaseError"
        type="error"
        :title="$gettext('Get release information error')"
        :message="getReleaseError"
        banner
      />
      <template v-else>
        <p>{{ $gettext('This will upgrade or reinstall the Nginx UI on %{nodeNames} to %{version}.', { nodeNames, version: data.name }) }}</p>

        <AAlert
          v-if="dryRun"
          type="info"
          class="mb-4"
          :message="$gettext('Dry run mode enabled')"
          banner
        />

        <div
          v-show="showLogContainer"
          class="mb-4"
        >
          <AProgress
            :stroke-color="progressStrokeColor"
            :percent="progressPercentComputed"
            :status="progressStatus"
          />

          <div
            ref="logContainer"
            class="core-upgrade-log-container"
          />
        </div>
        <div v-show="!showLogContainer && data.body">
          <h1 class="latest-version">
            {{ data.name }}
            <ATag
              v-if="channel === 'stable'"
              color="green"
            >
              {{ $gettext('Stable') }}
            </ATag>
            <ATag
              v-if="channel === 'prerelease'"
              color="blue"
            >
              {{ $gettext('Pre-release') }}
            </ATag>
          </h1>
          <div v-html="marked.parse(data.body)" />
        </div>

        <div class="flex justify-end">
          <AButton
            v-if="!showLogContainer"
            type="primary"
            @click="performUpgrade"
          >
            {{ $gettext('Perform') }}
          </AButton>
        </div>
      </template>
    </ASpin>
  </AModal>
</template>

<style scoped lang="less">
.dark {
  :deep(.core-upgrade-log-container) {
    background-color: rgba(0, 0, 0, 0.84);
  }
}

:deep(.core-upgrade-log-container) {
  height: 320px;
  overflow: scroll;
  background-color: #f3f3f3;
  border-radius: 4px;
  margin-top: 15px;
  padding: 10px;

  p {
    font-size: 12px;
    line-height: 1.3;
  }
}

.latest-version {
  display: flex;
  align-items: center;

  span.ant-tag {
    margin-left: 10px;
  }
}

:deep(h1) {
  font-size: 20px;
}

:deep(h2) {
  font-size: 18px;
}
</style>
