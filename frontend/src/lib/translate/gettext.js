import {translate} from 'vue-gettext'
import store from '@/lib/store'
import {availableLanguages} from '@/lib/translate/index'

let lang = window.navigator.language.replace('-', '_')
if(availableLanguages[lang] === undefined) {
    lang = lang.split('_')[0]
    if(availableLanguages[lang] === undefined)
        lang = 'en'
}
store.getters.current_language ||
store.commit('set_language', lang)

const config = {
    language: store.getters.current_language,
    getTextPluginSilent: true,
    getTextPluginMuteLanguages: [],
    silent: true,
}

// easygettext aliases
export const {
    gettext: $gettext, gettextInterpolate: $interpolate
} = translate

translate.initTranslations(store.state.settings.translations, config)

export default $gettext
