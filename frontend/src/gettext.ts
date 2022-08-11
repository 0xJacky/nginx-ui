import {createGettext} from 'vue3-gettext'
import translations from './language/translations.json'

export default createGettext({
    availableLanguages: {
        en: 'En',
        zh_CN: '简',
        zh_TW: '繁',
    },
    defaultLanguage: 'en',
    translations: translations,
    silent: true
})

export class useGettext {
}
