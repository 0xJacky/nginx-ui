/// <reference types="vite/client" />

// Extend Window interface
interface Window {
  inWorkspace?: boolean
}

/**
 * Versions of the runtime libraries the host shares with plugin bundles.
 * Injected by vite from the dependency ranges in package.json.
 */
declare const __NGINX_UI_SHARED_VERSIONS__: Record<string, string>

declare module '*.svg' {
  import type React from 'react'

  const content: React.FC<React.SVGProps<SVGElement>>
  export default content
}
