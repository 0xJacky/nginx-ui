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

declare module 'axios' {
  interface AxiosRequestConfig {
    crypto?: boolean
  }
}
