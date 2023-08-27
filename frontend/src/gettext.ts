import {createGettext} from 'vue3-gettext'

export default createGettext({
    availableLanguages: {
        en: 'En',
        zh_CN: '简',
        zh_TW: '繁',
        fr_FR: 'Fr',
        es: 'Es',
        ru_RU: 'Ru'
    },
    defaultLanguage: 'en',
    translations: {},
    silent: true
})

export class useGettext {
}
