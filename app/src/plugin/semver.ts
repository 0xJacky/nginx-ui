/**
 * Minimal semver range matcher for the shared runtime compatibility check.
 *
 * Supports `>=`, `>`, `<=`, `<`, `=`, `^`, `~`, exact versions, space
 * separated AND, and `||` OR. Prerelease and build metadata are ignored, so
 * `1.2.3-beta.1` is treated as `1.2.3`.
 */

export interface SemverParts {
  major: number
  minor: number
  patch: number
}

interface RangeAtom {
  operator: string
  parts: SemverParts
  /** How many of major/minor/patch the author actually wrote. */
  specified: number
}

const ATOM_PATTERN = /^(>=|<=|[><=^~])?v?(\d+|x|\*)(?:\.(\d+|x|\*))?(?:\.(\d+|x|\*))?$/i

const WILDCARDS = new Set(['*', 'x', 'X', ''])

function isWildcard(value: string | undefined): boolean {
  return value === undefined || WILDCARDS.has(value)
}

/** Strips build metadata and prerelease, then reads major/minor/patch. */
export function parseVersion(value: string): SemverParts | null {
  const core = String(value ?? '').trim().split('+')[0].split('-')[0]
  const match = /^v?(\d+)(?:\.(\d+))?(?:\.(\d+))?$/.exec(core)
  if (!match)
    return null

  return {
    major: Number(match[1]),
    minor: Number(match[2] ?? 0),
    patch: Number(match[3] ?? 0),
  }
}

export function compareVersions(a: SemverParts, b: SemverParts): number {
  if (a.major !== b.major)
    return a.major - b.major
  if (a.minor !== b.minor)
    return a.minor - b.minor
  return a.patch - b.patch
}

function parseAtom(value: string): RangeAtom | null {
  const match = ATOM_PATTERN.exec(value.trim())
  if (!match)
    return null

  const [, operator = '', rawMajor, rawMinor, rawPatch] = match

  // A leading wildcard matches everything regardless of the operator.
  if (isWildcard(rawMajor))
    return { operator: '*', parts: { major: 0, minor: 0, patch: 0 }, specified: 0 }

  let specified = 1
  if (!isWildcard(rawMinor))
    specified = 2
  if (!isWildcard(rawMinor) && !isWildcard(rawPatch))
    specified = 3

  return {
    operator,
    parts: {
      major: Number(rawMajor),
      minor: isWildcard(rawMinor) ? 0 : Number(rawMinor),
      patch: isWildcard(rawPatch) ? 0 : Number(rawPatch),
    },
    specified,
  }
}

/** Upper bound (exclusive) of a caret range, following the npm rules. */
function caretUpperBound({ parts, specified }: RangeAtom): SemverParts {
  if (parts.major > 0)
    return { major: parts.major + 1, minor: 0, patch: 0 }

  if (specified === 1)
    return { major: 1, minor: 0, patch: 0 }

  if (parts.minor > 0)
    return { major: 0, minor: parts.minor + 1, patch: 0 }

  if (specified === 3)
    return { major: 0, minor: 0, patch: parts.patch + 1 }

  return { major: 0, minor: 1, patch: 0 }
}

/** Upper bound (exclusive) of a tilde range. */
function tildeUpperBound({ parts, specified }: RangeAtom): SemverParts {
  if (specified === 1)
    return { major: parts.major + 1, minor: 0, patch: 0 }

  return { major: parts.major, minor: parts.minor + 1, patch: 0 }
}

/** Upper bound (exclusive) of a partial exact range such as `1.2`. */
function partialUpperBound({ parts, specified }: RangeAtom): SemverParts {
  if (specified === 1)
    return { major: parts.major + 1, minor: 0, patch: 0 }

  return { major: parts.major, minor: parts.minor + 1, patch: 0 }
}

function withinRange(version: SemverParts, lower: SemverParts, upper: SemverParts): boolean {
  return compareVersions(version, lower) >= 0 && compareVersions(version, upper) < 0
}

function matchAtom(version: SemverParts, atom: RangeAtom): boolean {
  const cmp = compareVersions(version, atom.parts)

  switch (atom.operator) {
    case '*':
      return true
    case '>=':
      return cmp >= 0
    case '>':
      return cmp > 0
    case '<=':
      return cmp <= 0
    case '<':
      return cmp < 0
    case '^':
      return withinRange(version, atom.parts, caretUpperBound(atom))
    case '~':
      return withinRange(version, atom.parts, tildeUpperBound(atom))
    default:
      // `=` and a bare version. A partial version behaves like `1.2.x`.
      if (atom.specified === 3)
        return cmp === 0
      return withinRange(version, atom.parts, partialUpperBound(atom))
  }
}

/** True when `version` satisfies every comparator of at least one OR group. */
export function satisfies(version: string, range: string): boolean {
  const parsed = parseVersion(version)
  if (!parsed)
    return false

  const trimmed = String(range ?? '').trim()
  if (!trimmed || trimmed === '*' || trimmed.toLowerCase() === 'x')
    return true

  return trimmed.split('||').some(group => {
    // Tolerate `>= 1.2.3`, which the split below would otherwise tear apart.
    const atoms = group.replace(/([<>=~^]+)\s+/g, '$1').trim().split(/\s+/).filter(Boolean)
    if (atoms.length === 0)
      return true

    return atoms.every(raw => {
      const atom = parseAtom(raw)
      return atom ? matchAtom(parsed, atom) : false
    })
  })
}
