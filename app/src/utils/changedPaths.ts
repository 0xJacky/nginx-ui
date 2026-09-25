type PlainObject = Record<string, unknown>

function isPlainObject(value: unknown): value is PlainObject {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

// Missing, null and empty string all mean "nothing entered" for a text field.
function isBlank(value: unknown) {
  return value === undefined || value === null || value === ''
}

function isEqualLeaf(before: unknown, after: unknown) {
  if (before === after)
    return true
  if (isBlank(before) && isBlank(after))
    return true
  if (Array.isArray(before) || Array.isArray(after))
    return JSON.stringify(before ?? []) === JSON.stringify(after ?? [])
  return false
}

/**
 * Lists the dot paths whose value differs between two settings objects.
 * Objects are walked recursively, arrays are compared as a whole.
 */
export function collectChangedPaths(before: unknown, after: unknown, prefix = ''): string[] {
  if (isPlainObject(before) || isPlainObject(after)) {
    const beforeObject = isPlainObject(before) ? before : {}
    const afterObject = isPlainObject(after) ? after : {}
    const keys = new Set([...Object.keys(beforeObject), ...Object.keys(afterObject)])
    const paths: string[] = []
    for (const key of keys) {
      const path = prefix ? `${prefix}.${key}` : key
      paths.push(...collectChangedPaths(beforeObject[key], afterObject[key], path))
    }
    return paths.sort()
  }

  return isEqualLeaf(before, after) ? [] : [prefix]
}

/**
 * Copies the values at the given dot paths from source into target.
 * Used to mark a subset of settings as saved without touching the rest.
 */
export function copyPaths<T extends object>(target: T, source: T, paths: string[]) {
  for (const path of paths) {
    const segments = path.split('.')
    const key = segments.pop()!
    let targetNode = target as PlainObject
    let sourceNode: PlainObject | undefined = source as PlainObject
    for (const segment of segments) {
      if (!isPlainObject(targetNode[segment]))
        targetNode[segment] = {}
      targetNode = targetNode[segment] as PlainObject
      sourceNode = isPlainObject(sourceNode?.[segment]) ? sourceNode![segment] as PlainObject : undefined
    }
    const value = sourceNode?.[key]
    targetNode[key] = value === undefined ? undefined : JSON.parse(JSON.stringify(value))
  }
}
