export interface ExternalNotifyConfigItem {
  key: string
  label: string
}

export interface ExternalNotifyConfig {
  name: () => string
  config: ExternalNotifyConfigItem[]
}
