import DirectiveEditor from './directive/DirectiveEditor.vue'
import LocationEditor from './LocationEditor.vue'
import LogEntry from './LogEntry.vue'
import NginxStatusAlert from './NginxStatusAlert.vue'
import NgxConfigEditor from './NgxConfigEditor.vue'
import NgxServer from './NgxServer.vue'
import NgxUpstream from './NgxUpstream.vue'
import { useNgxConfigStore } from './store'

export const If = 'if'
export const Server = 'server'
export const Location = 'location'
export const Upstream = 'upstream'
export const Http = 'http'
export const Stream = 'stream'
export const Include = 'include'

export {
  DirectiveEditor,
  LocationEditor,
  LogEntry,
  NginxStatusAlert,
  NgxServer,
  NgxUpstream,
  useNgxConfigStore,
}

export default NgxConfigEditor
