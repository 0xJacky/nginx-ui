<script setup lang="ts">
import { message } from 'ant-design-vue'
import config from '@/api/config'
import use2FAModal from '@/components/TwoFA/use2FAModal'

const props = defineProps<{
  dir?: string
}>()

const name = defineModel<string>('name', { default: '' })

const router = useRouter()

const modify = ref(false)
const buffer = ref('')
const loading = ref(false)

function clickModify() {
  buffer.value = name.value
  modify.value = true
}

function save() {
  loading.value = true
  const otpModal = use2FAModal()

  otpModal.open().then(() => {
    config.rename(props.dir!, name.value, buffer.value).then(() => {
      modify.value = false
      message.success($gettext('Renamed successfully'))
      router.push({
        path: `/config/${encodeURIComponent(buffer.value)}/edit`,
        query: {
          basePath: encodeURIComponent(props.dir!),
        },
      })
    }).finally(() => {
      loading.value = false
    })
  })
}
</script>

<template>
  <div v-if="!modify" class="flex items-center">
    <div class="mr-2">
      {{ name }}
    </div>
    <div>
      <AButton type="link" size="small" @click="clickModify">
        {{ $gettext('Rename') }}
      </AButton>
    </div>
  </div>
  <div v-else>
    <AInput v-model:value="buffer">
      <template #suffix>
        <AButton :disabled="buffer === name" type="link" size="small" :loading @click="save">
          {{ $gettext('Save') }}
        </AButton>
      </template>
    </AInput>
  </div>
</template>

<style scoped lang="less">

</style>
