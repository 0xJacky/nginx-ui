// uno.config.ts
import {
  defineConfig,
  presetAttributify,
  presetIcons,
  presetTypography,
  presetUno,
  presetWebFonts,
  transformerDirectives,
  transformerVariantGroup,
} from 'unocss'

export default defineConfig({
  shortcuts: [],
  rules: [],
  variants: [
    // 使用工具函数
    matcher => {
      if (!matcher.endsWith('!'))
        return matcher
      return {
        matcher: matcher.slice(0, -1),
        selector: s => `${s}!important`,
      }
    },
  ],
  theme: {
    colors: {
      // ...
    },
  },
  presets: [
    presetUno(),
    presetAttributify(),
    presetIcons({
      collections: {
        tabler: () => import('@iconify-json/tabler/icons.json').then(i => i.default),
      },
      extraProperties: {
        'display': 'inline-block',
        'height': '1.2em',
        'width': '1.2em',
        'vertical-align': 'text-bottom',
      },
    }),
    presetTypography(),
    presetWebFonts(),
  ],
  transformers: [
    transformerDirectives(),
    transformerVariantGroup(),
  ],
  content: {
    pipeline: {
      include: [
        // default
        /\.(vue|[jt]sx|ts)($|\?)/,

        // 参考：https://unocss.dev/guide/extracting#extracting-from-build-tools-pipeline
      ],

      // exclude files
      // exclude: []
    },
  },
})
