import { createApp } from 'vue'
import { createPinia } from 'pinia'
import piniaPluginPersistedstate from 'pinia-plugin-persistedstate'
import { autoAnimatePlugin } from '@formkit/auto-animate/vue'
import gettext from './gettext'
import App from './App.vue'
import router from './routes'
import { useSettingsStore } from '@/pinia'
import './style.css'

const pinia = createPinia()

const app = createApp(App)

pinia.use(piniaPluginPersistedstate)
app.use(pinia)
app.use(gettext)

// after pinia created
const settings = useSettingsStore()

gettext.current = settings.language || 'en'

app.use(router).use(autoAnimatePlugin).mount('#app')

export default app
