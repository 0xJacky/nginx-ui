<script setup lang="ts">
import type { Cert } from '@/api/cert'
import { SettingPanel, SettingRow } from '@/components/SettingPanel'
import ChangeCert from '@/views/site/site_edit/components/Cert/ChangeCert.vue'
import useSystemSettingsStore from '../store'

const systemSettingsStore = useSystemSettingsStore()
const { data } = storeToRefs(systemSettingsStore)

const isUnixListener = computed(() => !!data.value?.listener?.unix_socket)

function handleCertChange(certs: Cert[]) {
  if (certs.length > 0 && data.value?.server) {
    data.value.server.ssl_cert = certs[0].ssl_certificate_path
    data.value.server.ssl_key = certs[0].ssl_certificate_key_path
  }
}
</script>

<template>
  <div v-if="data?.server">
    <SettingPanel :title="$gettext('Listen Address')">
      <SettingRow
        :title="$gettext('Host')"
        path="server.host"
        config-file="server"
        :value="data.server.host || $gettext('All interfaces')"
      />
      <SettingRow
        :title="$gettext('Port')"
        path="server.port"
        config-file="server"
        :value="data.server.port"
      />
      <SettingRow
        v-if="isUnixListener"
        :title="$gettext('Unix Socket')"
        :description="$gettext('Nginx UI listens on this Unix socket instead of the TCP host and port.')"
        path="listener.unix_socket"
        config-file="listener"
        :value="data.listener?.unix_socket"
      />
      <SettingRow
        :title="$gettext('Run Mode')"
        path="server.run_mode"
        config-file="server"
        :value="data.server.run_mode"
      />
    </SettingPanel>

    <SettingPanel :title="$gettext('HTTPS')">
      <SettingRow
        :title="$gettext('Enable HTTPS')"
        :description="$gettext('Serves the web interface over HTTPS. Saving this change restarts Nginx UI and reloads the page.')"
        path="server.enable_https"
      >
        <ASwitch v-model:checked="data.server.enable_https" />
      </SettingRow>

      <template v-if="data.server.enable_https">
        <SettingRow
          :title="$gettext('Certificate')"
          :description="$gettext('Pick the certificate that secures the web interface.')"
          stacked
        >
          <ChangeCert
            selection-type="radio"
            @change="handleCertChange"
          />
        </SettingRow>
        <SettingRow
          :title="$gettext('SSL Certificate Path')"
          path="server.ssl_cert"
          :value="data.server.ssl_cert"
        />
        <SettingRow
          :title="$gettext('SSL Key Path')"
          path="server.ssl_key"
          :value="data.server.ssl_key"
        />
        <SettingRow
          :title="$gettext('Enable HTTP/2')"
          :description="$gettext('Enables HTTP/2 support with multiplexing and server push capabilities')"
          path="server.enable_h2"
        >
          <ASwitch v-model:checked="data.server.enable_h2" />
        </SettingRow>
        <SettingRow
          :title="$gettext('Enable HTTP/3')"
          :description="isUnixListener
            ? $gettext('HTTP/3 requires a UDP listener and is unavailable while Nginx UI listens on a Unix socket.')
            : $gettext('Enables HTTP/3 support based on QUIC protocol for best performance')"
          path="server.enable_h3"
        >
          <ASwitch
            v-model:checked="data.server.enable_h3"
            :disabled="isUnixListener"
          />
        </SettingRow>
        <AAlert
          type="info"
          :title="$gettext('Protocol configuration only takes effect when directly connecting. If using reverse proxy, please configure the protocol separately in the reverse proxy.')"
          show-icon
          class="my-3"
        />
      </template>
    </SettingPanel>
  </div>
</template>
