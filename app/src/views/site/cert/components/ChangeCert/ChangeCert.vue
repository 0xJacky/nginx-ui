<script setup lang="ts">
import type { Cert } from '@/api/cert'
import type { NgxDirective } from '@/api/ngx'
import type { Ref, WritableComputedRef } from 'vue'
import cert from '@/api/cert'
import StdTable from '@/components/StdDesign/StdDataDisplay/StdTable.vue'
import certColumns from '@/views/certificate/CertificateList/certColumns'

const emit = defineEmits(['change'])

const current_server_directives = inject('current_server_directives') as WritableComputedRef<NgxDirective[]>
const visible = ref(false)

function open() {
  visible.value = true
}

const records = ref([]) as Ref<Cert[]>
const selectedKeys = ref([])

async function ok() {
  // clear all ssl_certificate and ssl_certificate_key
  current_server_directives.value
    = current_server_directives.value
      .filter(v => v.directive !== 'ssl_certificate' && v.directive !== 'ssl_certificate_key')

  records.value.forEach(v => {
    current_server_directives?.value.push({
      directive: 'ssl_certificate',
      params: v.ssl_certificate_path,
    })
    current_server_directives?.value.push({
      directive: 'ssl_certificate_key',
      params: v.ssl_certificate_key_path,
    })
  })

  visible.value = false
  emit('change', records.value)

  records.value = []
  selectedKeys.value = []
}
</script>

<template>
  <div>
    <AButton @click="open">
      {{ $gettext('Change Certificate') }}
    </AButton>
    <AModal
      v-model:open="visible"
      :title="$gettext('Change Certificate')"
      :mask="false"
      width="800px"
      @ok="ok"
    >
      <StdTable
        v-model:selected-row-keys="selectedKeys"
        v-model:selected-rows="records"
        :api="cert"
        pithy
        :columns="certColumns"
        selection-type="checkbox"
      />
    </AModal>
  </div>
</template>

<style lang="less" scoped>

</style>
