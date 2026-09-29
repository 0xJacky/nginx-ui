import type { NginxUIGlobal, NginxUIPlugin } from './types'
import * as antdvIcons from '@antdv-next/icons'
import { http } from '@uozi-admin/request'
import * as vueuse from '@vueuse/core'
import * as antdvNext from 'antdv-next'
import * as pinia from 'pinia'
import * as vue from 'vue'
import * as vueRouter from 'vue-router'
import { openDnsCredentialEditor } from '@/components/DnsCredentialEditor/openDnsCredentialEditor'
import gettext from '@/gettext'
import version from '@/version.json'

/**
 * Definitions handed over by bundles that already executed. The loader takes
 * one out right after the matching `<script>` finished loading.
 */
const pendingPlugins = new Map<string, NginxUIPlugin>()

/** Called by a plugin bundle as `window.NginxUI.registerPlugin(id, definition)`. */
function registerPlugin(id: string, definition: NginxUIPlugin) {
  if (!id || typeof definition?.setup !== 'function') {
    console.warn('[plugin] registerPlugin ignored: an id and a setup function are required')
    return
  }

  pendingPlugins.set(id, definition)
}

/** Removes and returns the definition a bundle registered for `id`. */
export function takePendingPlugin(id: string): NginxUIPlugin | undefined {
  const definition = pendingPlugins.get(id)
  pendingPlugins.delete(id)
  return definition
}

/**
 * Publishes the host runtime on `window.NginxUI`.
 *
 * Plugin bundles are built as IIFEs whose externals map onto these namespaces,
 * so they run against the host's own Vue, router, store and UI library.
 */
export function installSharedRuntime() {
  const shared: NginxUIGlobal = {
    version: version.version,
    shared: {
      vue,
      vueRouter,
      pinia,
      antdvNext,
      antdvIcons,
      vueuse,
      gettext,
      http,
      versions: __NGINX_UI_SHARED_VERSIONS__,
      // Host dialogs for plugins. `openDnsCredentialEditor` lets a certificate
      // form create a DNS credential in place and select it afterwards.
      ui: {
        openDnsCredentialEditor,
      },
    },
    registerPlugin,
  }

  window.NginxUI = shared

  return shared
}
