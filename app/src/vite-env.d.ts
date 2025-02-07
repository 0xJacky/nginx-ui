/* eslint-disable ts/no-explicit-any */

/// <reference types="vite/client" />
/// <reference types="vite-svg-loader" />
/// <reference types="vue-dompurify-html" />
declare module '*.vue' {
  import type { DefineComponent } from 'vue'

  const component: DefineComponent<any, any, any>
  export default component
}

export { }
declare module '@vue/runtime-core' {
  interface ComponentCustomProperties {
    $gettext: (msgid: string, parameters?: {
      [key: string]: string
    }, disableHtmlEscaping?: boolean) => string
    $pgettext: (context: string, msgid: string, parameters?: {
      [key: string]: string
    }, disableHtmlEscaping?: boolean) => string
    $ngettext: (msgid: string, plural: string, n: number, parameters?: {
      [key: string]: string
    }, disableHtmlEscaping?: boolean) => string
    $npgettext: (context: string, msgid: string, plural: string, n: number, parameters?: {
      [key: string]: string
    }, disableHtmlEscaping?: boolean) => string
  }
}

declare module 'axios' {
  interface AxiosRequestConfig {
    crypto?: boolean
    skipErrHandling?: boolean
  }
}
