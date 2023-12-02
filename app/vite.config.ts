import { URL, fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import Components from 'unplugin-vue-components/vite'
import { AntDesignVueResolver } from 'unplugin-vue-components/resolvers'
import vueJsx from '@vitejs/plugin-vue-jsx'

import vitePluginBuildId from 'vite-plugin-build-id'
import svgLoader from 'vite-svg-loader'
import AutoImport from 'unplugin-auto-import/vite'
import DefineOptions from 'unplugin-vue-define-options/vite'

// https://vitejs.dev/config/
export default defineConfig({
  base: './',
  resolve: {
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
    Components({
      resolvers: [AntDesignVueResolver({ importStyle: false })],
      directoryAsNamespace: true,
    }),
    AutoImport({
      imports: ['vue', 'vue-router', 'pinia'],
      vueTemplate: true,
    }),
    DefineOptions(),
  ],
  css: {
    preprocessorOptions: {
      less: {
        modifyVars: {
          'border-radius-base': '5px',
        },
        javascriptEnabled: true,
      },
    },
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:9001/',
        changeOrigin: true,
        secure: false,
        ws: true,
      },
    },
  },
  build: {
    chunkSizeWarningLimit: 1000,
  },
})
