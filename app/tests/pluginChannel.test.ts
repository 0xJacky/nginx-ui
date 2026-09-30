import { describe, expect, test } from 'bun:test'

Object.assign(globalThis, { $gettext: (text: string) => text, $pgettext: (_context: string, text: string) => text })

const {
  channelDescription,
  channelHint,
  channelLabel,
  channelRank,
  compareVersions,
  effectiveChannel,
  entryChannel,
  followedChannel,
  hasStableRelease,
  heldByReleaseText,
  inferChannel,
  installableReleases,
  isDowngrade,
  isHeldByRelease,
  pluginChannel,
  releaseChannel,
} = await import('../src/views/system/plugins/channel')

describe('inferChannel', () => {
  test('a version without a prerelease part is stable', () => {
    expect(inferChannel('1.0.0')).toBe('stable')
    expect(inferChannel('v1.0.0+build-5')).toBe('stable')
    expect(inferChannel('')).toBe('stable')
    expect(inferChannel(undefined)).toBe('stable')
  })

  test('alpha, dev, nightly, snapshot, canary and preview are dev', () => {
    for (const version of ['1.0.0-alpha', '1.0.0-DEV.3', '1.0.0-nightly.20260930', '1.0.0-snapshot', '1.0.0-canary.2', '1.0.0-preview.1'])
      expect(inferChannel(version)).toBe('dev')
  })

  test('any other prerelease is beta', () => {
    for (const version of ['1.0.0-beta.1', 'v2.0.0-rc.1+build.5', '1.0.0-pre', '0.3.0-x.1', '1.0.0-alphabet'])
      expect(inferChannel(version)).toBe('beta')
    // Only the first identifier decides.
    expect(inferChannel('1.0.0-beta.alpha')).toBe('beta')
  })
})

describe('channelRank', () => {
  test('orders stable, beta and dev', () => {
    expect(channelRank('stable')).toBeLessThan(channelRank('beta'))
    expect(channelRank('beta')).toBeLessThan(channelRank('dev'))
    expect(channelRank(undefined)).toBe(0)
    expect(channelRank('nightly')).toBe(0)
  })
})

describe('releaseChannel', () => {
  test('follows the catalog channel, then the version', () => {
    expect(releaseChannel({ version: '1.0.0', channel: 'beta' })).toBe('beta')
    expect(releaseChannel({ version: '1.0.0-beta.1' })).toBe('beta')
    expect(releaseChannel({ version: '1.0.0-rc.1', channel: 'stable' })).toBe('stable')
    expect(releaseChannel({ version: '1.0.0' })).toBe('stable')
    expect(releaseChannel(undefined)).toBe('stable')
  })
})

describe('entryChannel', () => {
  test('is the computed channel, the beta stage or the release it installs', () => {
    expect(entryChannel({ channel: 'dev' })).toBe('dev')
    expect(entryChannel({ stage: 'beta' })).toBe('beta')
    expect(entryChannel({ installable_release: { version: '0.2.0-rc.1' } as never })).toBe('beta')
    expect(entryChannel({ stage: 'beta', installable_release: { version: '0.2.0-nightly.1' } as never })).toBe('dev')
    expect(entryChannel({ stage: 'production', installable_release: { version: '1.0.0' } as never })).toBe('stable')
    expect(entryChannel(undefined)).toBe('stable')
  })
})

describe('pluginChannel and followedChannel', () => {
  test('read the reported fields with the version as fallback', () => {
    expect(pluginChannel({ version: '1.0.0', channel: 'dev' })).toBe('dev')
    expect(pluginChannel({ version: '1.0.0-beta.2' })).toBe('beta')
    expect(pluginChannel({ version: '1.0.0' })).toBe('stable')
    expect(followedChannel({ followed_channel: 'beta' })).toBe('beta')
    expect(followedChannel({})).toBe('stable')
    expect(followedChannel(undefined)).toBe('stable')
  })
})

describe('effectiveChannel', () => {
  test('is the less stable of the chosen channel and the running release', () => {
    expect(effectiveChannel({ version: '1.0.0', channel: 'stable', followed_channel: 'stable' })).toBe('stable')
    expect(effectiveChannel({ version: '1.0.0', channel: 'stable', followed_channel: 'beta' })).toBe('beta')
    expect(effectiveChannel({ version: '1.0.0-beta.1', channel: 'beta', followed_channel: 'stable' })).toBe('beta')
    expect(effectiveChannel({ version: '1.0.0-nightly.1', channel: 'dev', followed_channel: 'beta' })).toBe('dev')
    // The reported value wins, an older node falls back to the fields.
    expect(effectiveChannel({ version: '1.0.0', effective_channel: 'dev' })).toBe('dev')
    expect(effectiveChannel({ version: '1.0.0-rc.1' })).toBe('beta')
    expect(effectiveChannel(undefined)).toBe('stable')
  })

  test('reports when the running release, not the choice, sets it', () => {
    expect(isHeldByRelease({ version: '1.0.0-beta.1', channel: 'beta', followed_channel: 'stable' })).toBe(true)
    expect(isHeldByRelease({ version: '1.0.0-beta.1', channel: 'beta', followed_channel: 'beta' })).toBe(false)
    expect(isHeldByRelease({ version: '1.0.0', channel: 'stable', followed_channel: 'beta' })).toBe(false)
    expect(isHeldByRelease({ version: '1.0.0', channel: 'stable', followed_channel: 'stable' })).toBe(false)
  })
})

describe('hasStableRelease', () => {
  test('looks for an installable stable release', () => {
    const betaOnly = { releases: [{ version: '0.1.0-beta.1' }, { version: '0.1.0-beta.2' }] } as never
    expect(hasStableRelease(betaOnly)).toBe(false)
    expect(hasStableRelease({ releases: [{ version: '0.1.0-beta.1' }, { version: '1.0.0' }] } as never)).toBe(true)
    // A withdrawn or unsupported stable release does not count.
    expect(hasStableRelease({ releases: [{ version: '1.0.0', yanked: true }, { version: '0.9.0-beta.1' }] } as never)).toBe(false)
    expect(hasStableRelease({ releases: [{ version: '1.0.0' }, { version: '0.9.0-beta.1' }], installable_versions: ['0.9.0-beta.1'] } as never)).toBe(false)
    expect(hasStableRelease(undefined)).toBe(false)
  })
})

describe('compareVersions', () => {
  test('orders by semantic version precedence', () => {
    expect(compareVersions('1.0.0', '1.0.1')).toBe(-1)
    expect(compareVersions('v2.0.0', '1.9.9')).toBe(1)
    expect(compareVersions('1.0.0+a', '1.0.0+b')).toBe(0)
    // A release sorts after its own prereleases.
    expect(compareVersions('1.1.0', '1.1.0-beta.3')).toBe(1)
    expect(compareVersions('1.1.0-beta.3', '1.1.0')).toBe(-1)
    expect(compareVersions('1.1.0-beta.3', '1.0.1')).toBe(1)
    // Numeric identifiers compare as numbers, so beta.10 is newer than beta.9.
    expect(compareVersions('1.1.0-beta.10', '1.1.0-beta.9')).toBe(1)
    expect(compareVersions('1.1.0-beta10', '1.1.0-beta9')).toBe(-1)
    expect(compareVersions('1.1.0-beta.2', '1.1.0-rc.1')).toBe(-1)
    expect(compareVersions('1.1.0-beta', '1.1.0-beta.1')).toBe(-1)
  })
})

describe('isDowngrade', () => {
  test('is true only for an older target', () => {
    expect(isDowngrade('1.1.0', '1.0.0')).toBe(true)
    expect(isDowngrade('1.1.0', '1.1.0-beta.1')).toBe(true)
    expect(isDowngrade('1.1.0', '1.2.0')).toBe(false)
    expect(isDowngrade('1.1.0', '1.1.0')).toBe(false)
    expect(isDowngrade(undefined, '1.0.0')).toBe(false)
    expect(isDowngrade('1.0.0', undefined)).toBe(false)
  })
})

describe('installableReleases', () => {
  const releases = [
    { version: '1.0.0' },
    { version: '1.2.0-beta.1' },
    { version: '1.1.0', yanked: true },
    { version: '1.1.0-beta.10' },
    { version: '1.1.0-beta.9' },
    { version: '0.9.0' },
  ] as never[]

  test('drops withdrawn releases and sorts the newest first', () => {
    expect(installableReleases({ releases }).map(item => item.version))
      .toEqual(['1.2.0-beta.1', '1.1.0-beta.10', '1.1.0-beta.9', '1.0.0', '0.9.0'])
  })

  test('keeps only the versions the node reports as installable', () => {
    expect(installableReleases({ releases, installable_versions: ['1.0.0', '1.1.0-beta.9'] }).map(item => item.version))
      .toEqual(['1.1.0-beta.9', '1.0.0'])
    expect(installableReleases(undefined)).toEqual([])
  })
})

describe('texts', () => {
  test('describe each channel without technical terms', () => {
    expect(channelLabel('stable')).toBe('Stable')
    expect(channelLabel('beta')).toBe('Beta')
    expect(channelLabel('dev')).toBe('Development')
    expect(channelHint('beta')).toContain('beta release')
    expect(channelHint('dev')).toContain('development release')
    expect(channelHint('stable')).toContain('stable release')
    expect(channelDescription('stable')).not.toBe(channelDescription('beta'))
    expect(channelDescription('dev')).toContain('all releases')
    expect(heldByReleaseText('beta')).toContain('until a stable release is available')
    expect(heldByReleaseText('dev')).toContain('until a stable release is available')
  })
})
