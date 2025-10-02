import { autoAnimatePlugin } from '@formkit/auto-animate/vue'
import { createCosyProConfig } from '@uozi-admin/curd'
import { createRequestConfig } from '@uozi-admin/request'
import { createPinia } from 'pinia'
import piniaPluginPersistedstate from 'pinia-plugin-persistedstate'
import { createApp } from 'vue'
import VueDOMPurifyHTML from 'vue-dompurify-html'
import { setupInterceptors } from '@/lib/http/interceptors'
import { initPWAThemeColor, watchThemeChanges } from '@/lib/pwa'
import { useSettingsStore } from '@/pinia'
import i18n from '../i18n.json'
import App from './App.vue'
import gettext from './gettext'
import router from './routes'
import '@uozi-admin/curd/dist/index.css'
import 'virtual:uno.css'

const pinia = createPinia()

const app = createApp(App)

function setupTranslations() {
  return Object.keys(i18n).reduce((acc, cur) => {
    acc[cur] = gettext.translations[cur]
    return acc
  }, {})
}

createRequestConfig({
  baseURL: './api',
})

pinia.use(piniaPluginPersistedstate)

app.use(pinia)
  .use(gettext)
  .use(VueDOMPurifyHTML, {
    hooks: {
      uponSanitizeElement: (node, data) => {
        if (node.tagName && node.tagName.toLowerCase() === 'think') {
          data.allowedTags.think = true
        }
      },
    },
  })
  .use(setupInterceptors)
  .use(createCosyProConfig({
    i18n: {
      legacy: false,
      locale: 'zh-CN',
      fallbackLocale: 'en-US',
      messages: setupTranslations(),
    },
    time: {
      timestamp: false,
    },
    selector: {
      omitZeroString: true,
    },
  }))

// after pinia created
const settings = useSettingsStore()

gettext.current = settings.language || 'en'

app.use(router).use(autoAnimatePlugin).mount('#app')

// Initialize PWA theme color functionality after app is mounted
nextTick(() => {
  initPWAThemeColor()
  watchThemeChanges()
})

export default app
