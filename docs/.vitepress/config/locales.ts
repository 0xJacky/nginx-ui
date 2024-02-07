import {enConfig} from './en'
import {zhCNConfig} from './zh_CN'
import {zhTWConfig} from './zh_TW'

const locales = {
  root: { label: 'English', lang: 'en', ...enConfig },
  'zh_CN': { label: '简体中文', lang: 'zh-CN', ...zhCNConfig },
  'zh_TW': { label: '繁體中文', lang: 'zh-TW', ...zhTWConfig }
}

export default locales
