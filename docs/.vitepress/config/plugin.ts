import { DefaultTheme } from 'vitepress'

type Locale = 'en' | 'zh_CN' | 'zh_TW'

interface Text {
  en: string
  zh_CN: string
  zh_TW: string
}

interface Page {
  link: string
  text: Text
}

interface Group {
  text: Text
  items: Page[]
}

// The plugin development guide, one tree for every locale
const groups: Group[] = [
  {
    text: { en: 'Getting Started', zh_CN: '开始', zh_TW: '開始' },
    items: [
      { link: 'overview', text: { en: 'Overview', zh_CN: '概览', zh_TW: '概覽' } },
      { link: 'quick-start', text: { en: 'Quick Start', zh_CN: '快速上手', zh_TW: '快速上手' } },
      { link: 'development', text: { en: 'Develop and Debug', zh_CN: '开发与调试', zh_TW: '開發與除錯' } },
    ],
  },
  {
    text: { en: 'Plugin Basics', zh_CN: '插件基础', zh_TW: '外掛基礎' },
    items: [
      { link: 'manifest', text: { en: 'Manifest', zh_CN: '清单文件', zh_TW: '清單檔案' } },
      { link: 'permissions', text: { en: 'Permissions and Security', zh_CN: '权限与安全', zh_TW: '權限與安全' } },
      { link: 'naming', text: { en: 'Naming', zh_CN: '命名规则', zh_TW: '命名規則' } },
      { link: 'versioning', text: { en: 'Versions and Compatibility', zh_CN: '版本与兼容性', zh_TW: '版本與相容性' } },
    ],
  },
  {
    text: { en: 'Plugin Process', zh_CN: '插件进程', zh_TW: '外掛進程' },
    items: [
      { link: 'lifecycle', text: { en: 'Lifecycle', zh_CN: '生命周期', zh_TW: '生命週期' } },
      { link: 'protocol', text: { en: 'Protocol', zh_CN: '通信协议', zh_TW: '通訊協定' } },
      { link: 'host-api', text: { en: 'Host API', zh_CN: '宿主 API', zh_TW: '宿主 API' } },
      { link: 'http', text: { en: 'HTTP Endpoints', zh_CN: 'HTTP 接口', zh_TW: 'HTTP 介面' } },
    ],
  },
  {
    text: { en: 'Web Interface', zh_CN: '网页界面', zh_TW: '網頁介面' },
    items: [
      { link: 'webapp', text: { en: 'Browser Bundle', zh_CN: '浏览器包', zh_TW: '瀏覽器套件' } },
      { link: 'slots', text: { en: 'Slots', zh_CN: '插槽', zh_TW: '插槽' } },
      { link: 'pages', text: { en: 'Static Pages', zh_CN: '静态页面', zh_TW: '靜態頁面' } },
    ],
  },
  {
    text: { en: 'Capabilities', zh_CN: '能力', zh_TW: '能力' },
    items: [
      { link: 'capabilities/dns01', text: { en: 'DNS-01', zh_CN: 'DNS-01', zh_TW: 'DNS-01' } },
      { link: 'capabilities/notify', text: { en: 'Notification Channels', zh_CN: '通知渠道', zh_TW: '通知通道' } },
      { link: 'capabilities/probe', text: { en: 'Health Checks', zh_CN: '健康检查', zh_TW: '健康檢查' } },
      { link: 'capabilities/mcp', text: { en: 'MCP Tools', zh_CN: 'MCP 工具', zh_TW: 'MCP 工具' } },
      { link: 'capabilities/storage', text: { en: 'Storage', zh_CN: '存储', zh_TW: '儲存' } },
      { link: 'capabilities/cert-deploy', text: { en: 'Certificate Deployment', zh_CN: '证书部署', zh_TW: '憑證部署' } },
      { link: 'capabilities/blocklist', text: { en: 'Blocklists', zh_CN: '封禁列表', zh_TW: '封鎖清單' } },
      { link: 'capabilities/discovery', text: { en: 'Upstream Discovery', zh_CN: '上游发现', zh_TW: '上游探索' } },
      { link: 'capabilities/log-sink', text: { en: 'Access Log Streaming', zh_CN: '访问日志推送', zh_TW: '存取日誌串流' } },
      { link: 'capabilities/content', text: { en: 'Templates and Translations', zh_CN: '模板与翻译', zh_TW: '範本與翻譯' } },
    ],
  },
  {
    text: { en: 'Publishing', zh_CN: '发布', zh_TW: '發佈' },
    items: [
      { link: 'packaging', text: { en: 'Packaging', zh_CN: '打包', zh_TW: '打包' } },
      { link: 'signing', text: { en: 'Signing and Trust', zh_CN: '签名与信任', zh_TW: '簽章與信任' } },
      { link: 'catalog', text: { en: 'Catalogs', zh_CN: '插件目录', zh_TW: '外掛目錄' } },
    ],
  },
  {
    text: { en: 'Reference', zh_CN: '参考', zh_TW: '參考' },
    items: [
      { link: 'rules', text: { en: 'Check Rules', zh_CN: '检查规则', zh_TW: '檢查規則' } },
    ],
  },
]

const navText: Text = { en: 'Plugins', zh_CN: '插件开发', zh_TW: '外掛開發' }

function base(locale: Locale) {
  return locale === 'en' ? '/plugin/' : `/${locale}/plugin/`
}

export function pluginNav(locale: Locale): DefaultTheme.NavItemWithLink {
  return { text: navText[locale], link: `${base(locale)}overview`, activeMatch: `^${base(locale)}` }
}

export function pluginSidebar(locale: Locale): DefaultTheme.SidebarMulti {
  return {
    [base(locale)]: groups.map(group => ({
      text: group.text[locale],
      collapsed: false,
      items: group.items.map(page => ({ text: page.text[locale], link: base(locale) + page.link })),
    })),
  }
}
