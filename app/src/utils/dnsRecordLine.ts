export function getDefaultDNSRecordLine(providerCode?: string): string | undefined {
  switch (providerCode?.trim().toLowerCase()) {
    case 'alidns':
      return 'default'
    case 'huaweicloud':
      return 'default_view'
    case 'tencentcloud':
      return '默认'
    default:
      return undefined
  }
}
