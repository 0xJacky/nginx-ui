import type { SavableSettingsSection } from '@/api/settings'
import { SAVABLE_SETTINGS_SECTIONS } from '@/api/settings'

// The preference tab each savable settings section is edited on.
const settingsSectionTab: Record<SavableSettingsSection, string> = {
  app: 'app',
  server: 'server',
  auth: 'auth',
  oidc: 'auth',
  cert: 'cert',
  http: 'http',
  node: 'node',
  openai: 'openai',
  logrotate: 'logrotate',
  nginx: 'nginx',
  site_check: 'health_check',
  upstream_check: 'health_check',
}

// The settings sections the Save button of a preference tab writes.
export function tabSettingsSections(tab: string): SavableSettingsSection[] {
  return SAVABLE_SETTINGS_SECTIONS.filter(section => settingsSectionTab[section] === tab)
}
