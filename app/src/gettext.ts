import { createGettext } from 'vue3-gettext'
import i18n from '../i18n.json'

export default createGettext({
  availableLanguages: i18n,
  defaultLanguage: 'en',
  translations: {},
  silent: true,
})

export class useGettext {}
