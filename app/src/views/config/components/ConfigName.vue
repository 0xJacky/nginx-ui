<script setup lang="ts">
import config from '@/api/config'
import use2FAModal from '@/components/TwoFA/use2FAModal'

const name = defineModel<string>('name', { default: '' })

const route = useRoute()
const router = useRouter()
const { message } = useGlobalApp()

const modify = ref(false)
const buffer = ref('')
const loading = ref(false)

function clickModify() {
  buffer.value = name.value
  modify.value = true
}

const { open: openOtpModal } = use2FAModal()

function save() {
  loading.value = true

  openOtpModal().then(() => {
    config.rename(route.query.basePath as string, name.value, buffer.value).then(() => {
      modify.value = false
      message.success($gettext('Renamed successfully'))
      router.push({
        path: `/config/${encodeURIComponent(buffer.value)}/edit`,
        query: {
          basePath: encodeURIComponent(route.query.basePath as string),
        },
      })
    }).finally(() => {
      loading.value = false
    })
  })
}
</script>

<template>
  <div>
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
  </div>
</template>

<style scoped lang="less">

</style>
