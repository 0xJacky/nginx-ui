import { readFileSync } from 'node:fs'
import { fileURLToPath, URL } from 'node:url'
import { AntdvNextResolver } from '@antdv-next/auto-import-resolver'
import vue from '@vitejs/plugin-vue'
import vueJsx from '@vitejs/plugin-vue-jsx'
import UnoCSS from 'unocss/vite'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import DefineOptions from 'unplugin-vue-define-options/vite'
import { defineConfig, loadEnv } from 'vite'
import vitePluginBuildId from 'vite-plugin-build-id'
import svgLoader from 'vite-svg-loader'

// Runtime libraries the host shares with plugin bundles through
// window.NginxUI.shared. Plugins declare the range they were built against in
// their manifest and the loader refuses a bundle that does not match.
const SHARED_RUNTIME_PACKAGES = [
  'vue',
  'vue-router',
  'pinia',
  'antdv-next',
  '@vueuse/core',
]

// The version a plugin is checked against is the one actually installed, not
// the lower bound declared in package.json, so a lockfile bump is visible to
// plugins without touching the manifest.
function resolveSharedVersions(): Record<string, string> {
  const pkg = JSON.parse(readFileSync(fileURLToPath(new URL('./package.json', import.meta.url)), 'utf-8'))
  const dependencies: Record<string, string> = pkg.dependencies ?? {}

  return SHARED_RUNTIME_PACKAGES.reduce((acc, name) => {
    try {
      const installed = JSON.parse(readFileSync(fileURLToPath(new URL(`./node_modules/${name}/package.json`, import.meta.url)), 'utf-8'))
      if (typeof installed.version === 'string') {
        acc[name] = installed.version
        return acc
      }
    }
    catch {
      // Fall back to the declared range when the package is not installed.
    }

    const range = dependencies[name]
    if (typeof range === 'string')
      acc[name] = range.replace(/^[\^~]/, '')

    return acc
  }, {} as Record<string, string>)
}

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')

  return {
    define: {
      __NGINX_UI_SHARED_VERSIONS__: JSON.stringify(resolveSharedVersions()),
    },
    base: './',
    resolve: {
      dedupe: [
        'vue',
        'vue-router',
        'pinia',
      ],
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
      extensions: [
        '.mjs',
        '.js',
        '.ts',
        '.jsx',
        '.tsx',
        '.json',
        '.vue',
        '.less',
      ],
    },
    plugins: [
      vue(),
      vueJsx(),
      vitePluginBuildId(),
      svgLoader(),
      UnoCSS(),
      Components({
        resolvers: [AntdvNextResolver()],
        directoryAsNamespace: true,
      }),
      AutoImport({
        imports: [
          'vue',
          'vue-router',
          'pinia',
          {
            '@/gettext': [
              '$gettext',
              '$pgettext',
              '$ngettext',
              '$npgettext',
            ],
          },
          {
            '@/language': ['T'],
          },
          {
            '@/composables/useGlobalApp': ['useGlobalApp'],
          },
          {
            'antdv-next': [
              'App',
            ],
          },
        ],
        vueTemplate: true,
        eslintrc: {
          enabled: true,
          filepath: '.eslint-auto-import.mjs',
        },
      }),
      DefineOptions(),
    ],
    css: {
      preprocessorOptions: {
        less: {
          javascriptEnabled: true,
        },
      },
    },
    server: {
      port: Number.parseInt(env.VITE_PORT) || 3002,
      proxy: {
        '/api': {
          target: env.VITE_PROXY_TARGET || 'http://localhost:9001',
          // Keep the browser Origin header. Rewriting it makes websocket
          // authentication fail when the dev server and backend ports differ.
          changeOrigin: false,
          secure: false,
          ws: true,
        },
        // Plugin bundles, icons and pages are static files served by the backend.
        '/plugins': {
          target: env.VITE_PROXY_TARGET || 'http://localhost:9001',
          changeOrigin: false,
          secure: false,
        },
      },
    },
    build: {
      chunkSizeWarningLimit: 1500,
    },
    optimizeDeps: {
      include: [
        'antdv-next',
      ],
    },
  }
})
