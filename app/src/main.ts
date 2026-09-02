import { autoAnimatePlugin } from '@formkit/auto-animate/vue'
import { createCosyProConfig } from '@uozi-admin/curd'
import { createRequestConfig } from '@uozi-admin/request'
import { createPinia } from 'pinia'
import piniaPluginPersistedstate from 'pinia-plugin-persistedstate'
import { createApp } from 'vue'
import VueDOMPurifyHTML from 'vue-dompurify-html'
import { getBrowserLanguage } from '@/lib/helper'
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
    // curd 6 accepts a reactive locale source, so its built-in copy follows the
    // language the user picked instead of being pinned to a single locale.
    locale: () => gettext.current,
    i18n: {
      legacy: false,
      locale: gettext.current,
      fallbackLocale: 'en',
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

// If the user has never selected a language, detect it from the browser and
// remember the choice as if they had set it manually. They can still change it
// later via the language selector. Fallback to English if no supported language
// matches.
if (!settings.language) {
  settings.set_language(getBrowserLanguage() || 'en')
}
else {
  gettext.current = settings.language
}

app.use(router).use(autoAnimatePlugin).mount('#app')

// Initialize PWA theme color functionality after app is mounted
nextTick(() => {
  initPWAThemeColor()
  watchThemeChanges()
})

export default app
