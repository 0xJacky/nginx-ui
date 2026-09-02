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

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')

  return {
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
          changeOrigin: true,
          secure: false,
          ws: true,
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
