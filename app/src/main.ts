import { useSettingsStore } from '@/pinia'
import { autoAnimatePlugin } from '@formkit/auto-animate/vue'
import { createPinia } from 'pinia'
import piniaPluginPersistedstate from 'pinia-plugin-persistedstate'
import { createApp } from 'vue'
import VueDOMPurifyHTML from 'vue-dompurify-html'
import App from './App.vue'
import gettext from './gettext'
import router from './routes'
import './style.css'

const pinia = createPinia()

const app = createApp(App)

pinia.use(piniaPluginPersistedstate)
app.use(pinia)
app.use(gettext)
app.use(VueDOMPurifyHTML)

// after pinia created
const settings = useSettingsStore()

gettext.current = settings.language || 'en'

app.use(router).use(autoAnimatePlugin).mount('#app')

export default app
