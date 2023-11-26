import {createApp} from 'vue'
import {createPinia} from 'pinia'
import gettext from './gettext'
import App from './App.vue'
import router from './routes'
import piniaPluginPersistedstate from 'pinia-plugin-persistedstate'
import {useSettingsStore} from '@/pinia'
import {autoAnimatePlugin} from '@formkit/auto-animate/vue'


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
