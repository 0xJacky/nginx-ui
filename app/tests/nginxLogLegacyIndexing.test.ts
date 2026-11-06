import { describe, expect, test } from 'bun:test'
import {
  LOG_ANALYTICS_PLUGIN_ID,
  logAnalyticsInstallRoute,
  needsLogAnalyticsPlugin,
} from '../src/views/nginx_log/legacyIndexing'

describe('needsLogAnalyticsPlugin', () => {
  test('asks for the plugin while the old indexing is on and the plugin is missing', () => {
    expect(needsLogAnalyticsPlugin(true, [])).toBe(true)
    expect(needsLogAnalyticsPlugin(true, ['com.example.other'])).toBe(true)
  })

  test('stays quiet once the plugin is installed or the old indexing was never on', () => {
    expect(needsLogAnalyticsPlugin(true, [LOG_ANALYTICS_PLUGIN_ID])).toBe(false)
    expect(needsLogAnalyticsPlugin(false, [])).toBe(false)
  })

  test('an unreadable plugin list does not count as installed', () => {
    expect(needsLogAnalyticsPlugin(true, null)).toBe(true)
    expect(needsLogAnalyticsPlugin(false, null)).toBe(false)
  })
})

describe('logAnalyticsInstallRoute', () => {
  test('points at the catalog entry of the plugin', () => {
    expect(logAnalyticsInstallRoute()).toEqual({
      path: '/system/plugins',
      query: { tab: 'marketplace', catalog: 'com.nginxui.log-analytics' },
    })
  })
})
