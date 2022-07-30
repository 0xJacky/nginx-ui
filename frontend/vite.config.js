import {defineConfig} from 'vite'
import path from 'path'
import {createVuePlugin} from 'vite-plugin-vue2'
import envCompatible from 'vite-plugin-env-compatible'
import {viteCommonjs} from '@originjs/vite-plugin-commonjs'

import vueTemplateBabelCompiler from 'vue-template-babel-compiler'
import styleImport from 'vite-plugin-style-import'
//import AntdMomentResolver from 'vite-plugin-antdv1-momentjs-resolver'

// https://vitejs.dev/config/
export default defineConfig({
    resolve: {
        alias: [
            {
                find: /^~/,
                replacement: ''
            },
            {
                find: '@',
                replacement: path.resolve(__dirname, 'src')
            }
        ],
        extensions: [
            '.mjs',
            '.js',
            '.ts',
            '.jsx',
            '.tsx',
            '.json',
            '.vue'
        ]
    },
    plugins: [
        createVuePlugin({
            jsx: true,
            vueTemplateOptions: {
                compiler: vueTemplateBabelCompiler
            }
        }),
        viteCommonjs(),
        envCompatible(),
        styleImport({
            libs: [
                {
                    libraryName: 'ant-design-vue',
                    esModule: true,
                    resolveStyle: (name) => {
                        return `ant-design-vue/es/${name}/style/index`
                    },
                }
            ],
        }),
        //AntdMomentResolver(),
    ],
    css: {
        preprocessorOptions: {
            css: {},
            postcss: {},
            less: {
                javascriptEnabled: true
            }
        }
    },
    server: {
        proxy: {
            '/api': {
                target: 'https://nginx.jackyu.cn/',
                changeOrigin: true,
                secure: false,
                ws: true,
            },
        },
    },
    build: {},

})
