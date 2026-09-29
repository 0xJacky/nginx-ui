<script setup lang="ts">
import type { Cert, CertificateInfo } from '@/api/cert'
import type { SiteStatus } from '@/api/site'
import CertInfo from '@/components/CertInfo/CertInfo.vue'
import { ConfigStatus } from '@/constants'
import EditorHTTPSCard from '../HTTPS/EditorHTTPSCard.vue'
import { extractServerDomains, isPendingTLSServer, sameStringList } from '../HTTPS/siteHTTPSState'
import { useSiteEditorStore } from '../SiteEditor/store'
import ChangeCert from './ChangeCert.vue'
import IssueCert from './IssueCert.vue'
import SelfSignedCert from './SelfSignedCert.vue'

const props = defineProps<{
  configName: string
  certInfo?: CertificateInfo[]
  siteStatus: SiteStatus
}>()

const editorStore = useSiteEditorStore()
const { curServer, curServerDirectives } = storeToRefs(editorStore)

const isSiteActive = computed(() => props.siteStatus === ConfigStatus.Enabled || props.siteStatus === ConfigStatus.Maintenance)

// A TLS server without certificate is set up by the backend-orchestrated
// HTTPS flow instead of the Let's Encrypt switch.
const isPendingTLS = computed(() => isPendingTLSServer(curServer.value))
// Keep the same array while the names are unchanged so the card does not
// reset domains the operator added.
const serverDomains = computed<string[]>(previous => {
  const next = extractServerDomains([curServer.value])
  return previous && sameStringList(previous, next) ? previous : next
})

function onHTTPSEnabled() {
  // The backend rewrote the site file; reload config and certificate info.
  editorStore.init(props.configName)
}

const changedCerts: Ref<Cert[]> = ref([])

// if certInfo update, clear changedCerts
watch(() => props.certInfo, () => {
  changedCerts.value = []
})

function handleCertChange(certs: Cert[]) {
  changedCerts.value = certs

  // Update NgxDirective
  if (curServerDirectives.value) {
    // Filter out existing certificate configurations
    const filteredDirectives = curServerDirectives.value
      .filter(v => v.directive !== 'ssl_certificate' && v.directive !== 'ssl_certificate_key')

    // Add new certificate configuration
    const newDirectives = [...filteredDirectives]

    certs.forEach(cert => {
      newDirectives.push({
        directive: 'ssl_certificate',
        params: cert.ssl_certificate_path,
      })
      newDirectives.push({
        directive: 'ssl_certificate_key',
        params: cert.ssl_certificate_key_path,
      })
    })

    // Update directives
    curServerDirectives.value = newDirectives
  }
}
</script>

<template>
  <div>
    <h3 v-if="certInfo?.length">
      {{ $ngettext('Certificate Status', 'Certificates Status', certInfo?.length || 1) }}
    </h3>

    <ARow
      v-if="certInfo?.length"
      :gutter="[16, 16]"
      class="mb-4"
    >
      <ACol
        v-for="(c, index) in certInfo"
        :key="index"
        :xs="24"
        :sm="12"
      >
        <CertInfo :cert="c" />
      </ACol>
    </ARow>

    <template v-if="changedCerts.length > 0">
      <h3>
        {{ $ngettext('Changed Certificate', 'Changed Certificates', changedCerts?.length || 1) }}
      </h3>
      <ARow
        :gutter="[16, 16]"
        class="mb-4"
      >
        <ACol
          v-for="(c, index) in changedCerts"
          :key="index"
          :xs="24"
          :sm="12"
        >
          <CertInfo :cert="c.certificate_info" />
        </ACol>
      </ARow>
    </template>

    <EditorHTTPSCard
      v-if="isSiteActive && isPendingTLS"
      class="mb-4"
      compact
      collapsible
      :domains="serverDomains"
      @success="onHTTPSEnabled"
    />

    <!-- The HTTPS card's "Existing certificate" method already picks one for
         a pending TLS server of an active site. -->
    <ChangeCert
      v-if="!(isSiteActive && isPendingTLS)"
      @change="handleCertChange"
    />

    <IssueCert
      v-if="isSiteActive && !isPendingTLS"
      :config-name
    />
    <SelfSignedCert v-if="isSiteActive" />
  </div>
</template>

<style scoped>

</style>
