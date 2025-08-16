// Type definitions for dashboard components

export interface URLStatItem {
  url: string
  visits: number
  percent: number
}

export interface BrowserStatItem {
  browser: string
  count: number
  percent: number
}

export interface OSStatItem {
  os: string
  count: number
  percent: number
}

export interface DeviceStatItem {
  device: string
  count: number
  percent: number
}
