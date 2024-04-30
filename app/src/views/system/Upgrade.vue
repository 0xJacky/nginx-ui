<script setup lang="ts">
import type { Ref } from 'vue'
import dayjs from 'dayjs'
import { marked } from 'marked'

import { message } from 'ant-design-vue'
import { useRoute } from 'vue-router'
import websocket from '@/lib/websocket'
import version from '@/version.json'
import type { RuntimeInfo } from '@/api/upgrade'
import upgrade from '@/api/upgrade'

const route = useRoute()
const data = ref({}) as Ref<RuntimeInfo>
const last_check = ref('')
const loading = ref(false)
const channel = ref('stable')

const progressStrokeColor = {
  from: '#108ee9',
  to: '#87d068',
}

const modalVisible = ref(false)
const progressPercent = ref(0)
const progressStatus = ref('active') as Ref<'normal' | 'active' | 'success' | 'exception'>
const modalClosable = ref(false)
const get_release_error = ref(false)

const progressPercentComputed = computed(() => {
  return Number.parseFloat(progressPercent.value.toFixed(1))
})

function get_latest_release() {
  loading.value = true
  data.value.body = ''
  upgrade.get_latest_release(channel.value).then(r => {
    data.value = r
    last_check.value = dayjs().format('YYYY-MM-DD HH:mm:ss')
  }).catch(e => {
    get_release_error.value = e?.message
    message.error(e?.message ?? $gettext('Server error'))
  }).finally(() => {
    loading.value = false
  })
}

get_latest_release()

watch(channel, get_latest_release)

const is_latest_ver = computed(() => {
  return data.value.name === `v${version.version}`
})

const logContainer = ref()

function log(msg: string) {
  const para = document.createElement('p')

  para.appendChild(document.createTextNode($gettext(msg)))

  logContainer.value.appendChild(para)

  logContainer.value.scroll({ top: 320, left: 0, behavior: 'smooth' })
}

const dry_run = computed(() => {
  return !!route.query.dry_run
})

async function perform_upgrade() {
  progressStatus.value = 'active'
  modalClosable.value = false
  modalVisible.value = true
  progressPercent.value = 0
  logContainer.value.innerHTML = ''

  log($gettext('Upgrading Nginx UI, please wait...'))

  const ws = websocket('/api/upgrade/perform', false)

  let last = 0

  ws.onopen = () => {
    ws.send(JSON.stringify({
      dry_run: dry_run.value,
      channel: channel.value,
    }))
  }

  let is_fail = false

  ws.onmessage = async m => {
    const r = JSON.parse(m.data)
    if (r.message)
      log(r.message)
    console.log(r.status)
    switch (r.status) {
      case 'info':
        progressPercent.value += 10
        break
      case 'progress':
        progressPercent.value += (r.progress - last) / 2
        last = r.progress
        break
      case 'error':
        progressStatus.value = 'exception'
        modalClosable.value = true
        is_fail = true
        break
      default:
        modalClosable.value = true
        break
    }
  }

  ws.onclose = async () => {
    if (is_fail)
      return

    const t = setInterval(() => {
      upgrade.current_version().then(() => {
        clearInterval(t)
        progressStatus.value = 'success'
        progressPercent.value = 100
        modalClosable.value = true
        log('Upgraded successfully')

        setInterval(() => {
          location.reload()
        }, 1000)
      })
    }, 2000)
  }
}
</script>

<template>
  <ACard :title="$gettext('Upgrade')">
    <AModal
      v-model:open="modalVisible"
      :title="$gettext('Core Upgrade')"
      :mask-closable="false"
      :footer="null"
      :closable="modalClosable"
      force-render
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
    </AModal>
    <div class="upgrade-container">
      <p>{{ $gettext('You can check Nginx UI upgrade at this page.') }}</p>
      <h3>{{ $gettext('Current Version') }}: v{{ version.version }}</h3>
      <template v-if="get_release_error">
        <AAlert
          type="error"
          :title="$gettext('Get release information error')"
          :message="get_release_error"
          banner
        />
      </template>
      <template v-else>
        <p>{{ $gettext('OS') }}: {{ data.os }}</p>
        <p>{{ $gettext('Arch') }}: {{ data.arch }}</p>
        <p>{{ $gettext('Executable Path') }}: {{ data.ex_path }}</p>
        <p>
          {{ $gettext('Last checked at') }}: {{ last_check }}
          <AButton
            type="link"
            :loading="loading"
            @click="get_latest_release"
          >
            {{ $gettext('Check again') }}
          </AButton>
        </p>
        <AFormItem :label="$gettext('Channel')">
          <ASelect v-model:value="channel">
            <ASelectOption key="stable">
              {{ $gettext('Stable') }}
            </ASelectOption>
            <ASelectOption key="prerelease">
              {{ $gettext('Pre-release') }}
            </ASelectOption>
          </ASelect>
        </AFormItem>
        <template v-if="!loading">
          <AAlert
            v-if="is_latest_ver"
            type="success"
            :message="$gettext('You are using the latest version')"
            banner
          />
          <AAlert
            v-else
            type="info"
            :message="$gettext('New version released')"
            banner
          />
          <template v-if="dry_run">
            <br>
            <AAlert
              type="info"
              :message="$gettext('Dry run mode enabled')"
              banner
            />
          </template>
          <div class="control-btn">
            <ASpace>
              <AButton
                v-if="is_latest_ver"
                type="primary"
                ghost
                @click="perform_upgrade"
              >
                {{ $gettext('Reinstall') }}
              </AButton>
              <AButton
                v-else
                type="primary"
                ghost
                @click="perform_upgrade"
              >
                {{ $gettext('Upgrade') }}
              </AButton>
            </ASpace>
          </div>
        </template>
      </template>
      <template v-if="data.body">
        <h2 class="latest-version">
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
        </h2>

        <h3>{{ $gettext('Release Note') }}</h3>
        <div v-html="marked.parse(data.body)" />
      </template>
    </div>
  </ACard>
</template>

<style lang="less">
.dark {
  .core-upgrade-log-container {
    background-color: rgba(0, 0, 0, 0.84);
  }
}

.core-upgrade-log-container {
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
</style>

<style lang="less" scoped>
.upgrade-container {
  width: 100%;
  max-width: 600px;
  margin: 0 auto;
  padding: 0 10px;
}

.control-btn {
  margin: 15px 0;
}

.latest-version {
  display: flex;
  align-items: center;

  span.ant-tag {
    margin-left: 10px;
  }
}
</style>
