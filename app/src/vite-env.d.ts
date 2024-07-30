/// <reference types="vite/client" />
/// <reference types="vite-svg-loader" />
import { ComponentCustomProperties } from '@vue/runtime-core'
declare module '*.vue' {
  import type { DefineComponent } from 'vue'

  const component: DefineComponent<{}, {}, any>
  export default component
}

export { }
declare module '@vue/runtime-core' {
  interface ComponentCustomProperties {
    $gettext: (msgid: string, parameters?: {
      [key: string]: string;
    }, disableHtmlEscaping?: boolean) => string;
    $pgettext: (context: string, msgid: string, parameters?: {
      [key: string]: string;
    }, disableHtmlEscaping?: boolean) => string;
    $ngettext: (msgid: string, plural: string, n: number, parameters?: {
      [key: string]: string;
    }, disableHtmlEscaping?: boolean) => string;
    $npgettext: (context: string, msgid: string, plural: string, n: number, parameters?: {
      [key: string]: string;
    }, disableHtmlEscaping?: boolean) => string;
  }
}
