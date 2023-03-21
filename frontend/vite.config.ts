import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'
import {createHtmlPlugin} from 'vite-plugin-html'
import Components from 'unplugin-vue-components/vite'
import {AntDesignVueResolver} from 'unplugin-vue-components/resolvers'
import {fileURLToPath, URL} from 'url'
import vueJsx from '@vitejs/plugin-vue-jsx'
import vitePluginBuildId from 'vite-plugin-build-id'
import svgLoader from 'vite-svg-loader'

// https://vitejs.dev/config/
export default defineConfig({
    base: './',
    resolve: {
        alias: {
            '@': fileURLToPath(new URL('./src', import.meta.url))
        },
        extensions: [
            '.mjs',
            '.js',
            '.ts',
            '.jsx',
            '.tsx',
            '.json',
            '.vue',
            '.less'
        ]
    },
    plugins: [vue(), vueJsx(), vitePluginBuildId(), svgLoader(),
        Components({
            resolvers: [AntDesignVueResolver({importStyle: false})],
            directoryAsNamespace: true
        }),
        createHtmlPlugin({
            minify: true,
            /**
             * After writing entry here, you will not need to add script tags in `index.html`, the original tags need to be deleted
             * @default src/main.ts
             */
            entry: '/src/main.ts',
            /**
             * If you want to store `index.html` in the specified folder, you can modify it, otherwise no configuration is required
             * @default index.html
             */
            template: 'index.html',

            /**
             * Data that needs to be injected into the index.html ejs template
             */
            inject: {
                data: {
                    title: 'Nginx UI'
                }
            }
        })
    ],
    css: {
        preprocessorOptions: {
            less: {
                modifyVars: {
                    'border-radius-base': '4px'
                },
                javascriptEnabled: true
            }
        }
    },
    server: {
        proxy: {
            '/api': {
                target: 'http://127.0.0.1:9001/',
                changeOrigin: true,
                secure: false,
                ws: true
            }
        }
    },
    build: {
        chunkSizeWarningLimit: 600
    }
})
