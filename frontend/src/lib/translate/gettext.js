import {translate} from 'vue-gettext'
import store from '@/lib/store'
import {availableLanguages} from '@/lib/translate/index'

let lang = window.navigator.language
if (!lang.includes('zh')) {
    lang = lang.split('-')[0]
} else {
    lang = lang.replace('-', '_')
}
store.getters.current_language ||
store.commit('set_language', availableLanguages[lang] ? lang : 'en')

const config = {
    language: store.getters.current_language,
    getTextPluginSilent: true,
    getTextPluginMuteLanguages: [],
    silent: true,
}

// easygettext aliases
const {
    gettext: $gettext,
} = translate

translate.initTranslations(store.state.settings.translations, config)

export default $gettext
