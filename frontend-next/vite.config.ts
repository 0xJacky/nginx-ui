import {defineConfig} from 'vite'
import vue from '@vitejs/plugin-vue'
import {createHtmlPlugin} from 'vite-plugin-html'
import Components from 'unplugin-vue-components/vite'
import {AntDesignVueResolver} from 'unplugin-vue-components/resolvers'
import {themePreprocessorPlugin, themePreprocessorHmrPlugin} from "@zougt/vite-plugin-theme-preprocessor";
import { fileURLToPath, URL } from "url"
import path from 'path'

// https://vitejs.dev/config/
export default defineConfig({
    resolve: {
        alias: {
            "@": fileURLToPath(new URL("./src", import.meta.url)),
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
    plugins: [vue(),
        Components({
            resolvers: [AntDesignVueResolver({importStyle: false})]
        }),
        themePreprocessorPlugin({
            less: {
                multipleScopeVars: [
                    {
                        scopeName: "theme-default",
                        path: path.resolve("./src/style.less"),
                    },
                    {
                        scopeName: "theme-dark",
                        path: path.resolve("./src/dark.less"),
                    },
                ],
                // css中不是由主题色变量生成的颜色，也让它抽取到主题css内，可以提高权重
                includeStyleWithColors: [
                    {
                        color: "#ffffff",
                        // 排除属性
                        // excludeCssProps:["background","background-color"]
                        // 排除选择器
                        // excludeSelectors: [
                        //   ".ant-btn-link:hover, .ant-btn-link:focus, .ant-btn-link:active",
                        // ],
                    },
                    {
                        color: ["transparent","none"],
                    },
                ],
            },
        }),
        themePreprocessorHmrPlugin(),
        createHtmlPlugin({
            minify: true,
            /**
             * After writing entry here, you will not need to add script tags in `index.html`, the original tags need to be deleted
             * @default src/main.ts
             */
            entry: 'src/main.ts',
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
                    title: 'Nginx UI',
                },
            },
        }),
    ],
    css: {
        preprocessorOptions: {
            less: {
                javascriptEnabled: true,
            }
        },
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
})
