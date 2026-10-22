import type { PluginChannel, PluginInfo } from '@/api/plugin'
import type { CatalogEntry, CatalogRelease } from '@/api/plugin_marketplace'

export type { PluginChannel }

export const PLUGIN_CHANNELS: PluginChannel[] = ['stable', 'beta', 'dev']

// Prerelease identifiers that mean the dev channel, any other prerelease is beta.
const DEV_IDENTIFIERS = new Set(['alpha', 'dev', 'nightly', 'snapshot', 'canary', 'preview'])

// A semver prerelease part follows the core version after a dash, as in 1.0.0-beta.1.
const PRERELEASE = /^v?\d+\.\d+\.\d+-([0-9a-z-][0-9a-z.-]*)/i

export function isPluginChannel(value: unknown): value is PluginChannel {
  return value === 'stable' || value === 'beta' || value === 'dev'
}

/** Rank of a channel, a higher rank is less stable. Unknown values count as stable. */
export function channelRank(channel?: string): number {
  return Math.max(0, PLUGIN_CHANNELS.indexOf(channel as PluginChannel))
}

/** The channel a version belongs to when nothing names one. */
export function inferChannel(version?: string): PluginChannel {
  const match = PRERELEASE.exec((version ?? '').trim().split('+')[0])
  if (!match)
    return 'stable'
  const first = match[1].split('.')[0].toLowerCase()
  return DEV_IDENTIFIERS.has(first) ? 'dev' : 'beta'
}

/** Channel of a catalog release: the one the catalog names, else the one its version implies. */
export function releaseChannel(release?: Pick<CatalogRelease, 'version' | 'channel'>): PluginChannel {
  if (!release)
    return 'stable'
  return isPluginChannel(release.channel) ? release.channel : inferChannel(release.version)
}

/** Channel of a marketplace entry: the release this node would install. */
export function entryChannel(entry?: Pick<CatalogEntry, 'channel' | 'installable_release'>): PluginChannel {
  if (!entry)
    return 'stable'
  if (isPluginChannel(entry.channel))
    return entry.channel
  return entry.installable_release ? releaseChannel(entry.installable_release) : 'stable'
}

/** Channel of the release an installed plugin runs. */
export function pluginChannel(plugin?: Pick<PluginInfo, 'channel' | 'version'>): PluginChannel {
  if (!plugin)
    return 'stable'
  return isPluginChannel(plugin.channel) ? plugin.channel : inferChannel(plugin.version)
}

/** Channel the person chose for an installed plugin, stable by default. */
export function followedChannel(plugin?: Pick<PluginInfo, 'followed_channel'>): PluginChannel {
  return isPluginChannel(plugin?.followed_channel) ? plugin.followed_channel : 'stable'
}

/** Channel updates of an installed plugin come from: the less stable of the chosen channel and the one it runs. */
export function effectiveChannel(plugin?: Pick<PluginInfo, 'channel' | 'version' | 'followed_channel' | 'effective_channel'>): PluginChannel {
  if (!plugin)
    return 'stable'
  if (isPluginChannel(plugin.effective_channel))
    return plugin.effective_channel
  const chosen = followedChannel(plugin)
  const running = pluginChannel(plugin)
  return channelRank(running) > channelRank(chosen) ? running : chosen
}

/** True when the running release, and not the person's choice, sets where updates come from. */
export function isHeldByRelease(plugin?: Pick<PluginInfo, 'channel' | 'version' | 'followed_channel' | 'effective_channel'>): boolean {
  return channelRank(effectiveChannel(plugin)) > channelRank(followedChannel(plugin))
}

export function channelLabel(channel: PluginChannel): string {
  switch (channel) {
    case 'beta':
      return $gettext('Beta')
    case 'dev':
      return $gettext('Development')
    default:
      return $gettext('Stable')
  }
}

/** Tooltip of the tag, in plain words. */
export function channelHint(channel: PluginChannel): string {
  switch (channel) {
    case 'beta':
      return $gettext('This is a beta release. Features may change, and some functions may not work as expected.')
    case 'dev':
      return $gettext('This is a development release. It may be incomplete or unstable.')
    default:
      return $gettext('This is a stable release.')
  }
}

/** Says what an installation gets while its release, not its choice, sets the channel. */
export function heldByReleaseText(channel: PluginChannel): string {
  return channel === 'dev'
    ? $gettext('Development releases are received until a stable release is available.')
    : $gettext('Beta releases are received until a stable release is available.')
}

/** Explains what following a channel means. */
export function channelDescription(channel: PluginChannel): string {
  switch (channel) {
    case 'beta':
      return $gettext('Receive beta and stable releases.')
    case 'dev':
      return $gettext('Receive all releases, including development releases.')
    default:
      return $gettext('Receive stable releases only.')
  }
}

/** Compares two versions by semantic version precedence, returning -1, 0 or 1. */
export function compareVersions(a: string, b: string): number {
  const left = parseParts(a)
  const right = parseParts(b)
  for (let i = 0; i < 3; i++) {
    if (left.core[i] !== right.core[i])
      return left.core[i] < right.core[i] ? -1 : 1
  }
  return comparePrerelease(left.pre, right.pre)
}

function parseParts(value: string) {
  const [withoutBuild] = value.trim().replace(/^v/, '').split('+')
  const dash = withoutBuild.indexOf('-')
  const core = dash < 0 ? withoutBuild : withoutBuild.slice(0, dash)
  const pre = dash < 0 ? '' : withoutBuild.slice(dash + 1)
  const numbers = core.split('.').map(part => Number.parseInt(part, 10) || 0)
  return { core: [numbers[0] ?? 0, numbers[1] ?? 0, numbers[2] ?? 0], pre }
}

function comparePrerelease(a: string, b: string): number {
  if (a === b)
    return 0
  // A release outranks any of its prereleases.
  if (a === '')
    return 1
  if (b === '')
    return -1

  const left = a.split('.')
  const right = b.split('.')
  for (let i = 0; i < Math.min(left.length, right.length); i++) {
    if (left[i] === right[i])
      continue
    const leftNumeric = /^\d+$/.test(left[i])
    const rightNumeric = /^\d+$/.test(right[i])
    if (leftNumeric && rightNumeric)
      return Number(left[i]) < Number(right[i]) ? -1 : 1
    // Numeric identifiers rank below the others.
    if (leftNumeric)
      return -1
    if (rightNumeric)
      return 1
    return left[i] < right[i] ? -1 : 1
  }
  return Math.sign(left.length - right.length)
}

/** True when installing target over installed steps back to an older version. */
export function isDowngrade(installed?: string, target?: string): boolean {
  return Boolean(installed) && Boolean(target) && compareVersions(target!, installed!) < 0
}

/** True when the entry has a stable release this node can install. */
export function hasStableRelease(entry?: Pick<CatalogEntry, 'releases' | 'installable_versions'>): boolean {
  return installableReleases(entry).some(release => releaseChannel(release) === 'stable')
}

/** The releases of an entry this node can install, newest first. */
export function installableReleases(entry?: Pick<CatalogEntry, 'releases' | 'installable_versions'>): CatalogRelease[] {
  if (!entry)
    return []
  const versions = entry.installable_versions
  const list = (entry.releases ?? []).filter(release => !release.yanked
    && (versions ? versions.includes(release.version) : true))
  return list.sort((a, b) => compareVersions(b.version, a.version))
}
