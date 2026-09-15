/**
 * Punycode decoding for display purposes.
 *
 * The backend persists internationalized domains in their canonical ASCII form
 * (`xn--fsq.example.com`), which is unambiguous but unreadable for the people who
 * typed `例.example.com`. These helpers decode that form back for display only —
 * every value sent to the API stays exactly as the backend stored it.
 *
 * Implements the decoding half of RFC 3492.
 */

const BASE = 36
const T_MIN = 1
const T_MAX = 26
const SKEW = 38
const DAMP = 700
const INITIAL_BIAS = 72
const INITIAL_N = 128
const DELIMITER = '-'
const ACE_PREFIX = 'xn--'
const MAX_INT = 0x7FFFFFFF

/** Maps a basic code point to its digit value, or -1 when it is not a digit. */
function digitValue(codePoint: number): number {
  if (codePoint >= 0x41 && codePoint <= 0x5A)
    return codePoint - 0x41 // A-Z
  if (codePoint >= 0x61 && codePoint <= 0x7A)
    return codePoint - 0x61 // a-z
  if (codePoint >= 0x30 && codePoint <= 0x39)
    return codePoint - 0x30 + 26 // 0-9
  return -1
}

function adaptBias(delta: number, numPoints: number, firstTime: boolean): number {
  let scaled = firstTime ? Math.floor(delta / DAMP) : delta >> 1
  scaled += Math.floor(scaled / numPoints)

  let k = 0
  while (scaled > ((BASE - T_MIN) * T_MAX) >> 1) {
    scaled = Math.floor(scaled / (BASE - T_MIN))
    k += BASE
  }

  return k + Math.floor(((BASE - T_MIN + 1) * scaled) / (scaled + SKEW))
}

/**
 * Decodes the punycode payload of a single label, returning undefined when the
 * input is not well-formed punycode.
 */
function decodePunycode(input: string): string | undefined {
  const output: number[] = []

  // Basic code points are copied verbatim up to the last delimiter.
  const lastDelimiter = input.lastIndexOf(DELIMITER)
  if (lastDelimiter > 0) {
    for (let index = 0; index < lastDelimiter; index++) {
      const codePoint = input.charCodeAt(index)
      if (codePoint >= 0x80)
        return undefined
      output.push(codePoint)
    }
  }

  let n = INITIAL_N
  let i = 0
  let bias = INITIAL_BIAS

  let index = lastDelimiter > 0 ? lastDelimiter + 1 : 0
  if (index >= input.length)
    return undefined

  while (index < input.length) {
    const oldI = i
    let w = 1

    for (let k = BASE; ; k += BASE) {
      if (index >= input.length)
        return undefined

      const digit = digitValue(input.charCodeAt(index++))
      if (digit < 0)
        return undefined
      if (digit > Math.floor((MAX_INT - i) / w))
        return undefined

      i += digit * w

      const t = k <= bias ? T_MIN : (k >= bias + T_MAX ? T_MAX : k - bias)
      if (digit < t)
        break

      if (w > Math.floor(MAX_INT / (BASE - t)))
        return undefined
      w *= BASE - t
    }

    const outLength = output.length + 1
    bias = adaptBias(i - oldI, outLength, oldI === 0)

    if (Math.floor(i / outLength) > MAX_INT - n)
      return undefined
    n += Math.floor(i / outLength)
    i %= outLength

    // Lone surrogates and out-of-range code points are not displayable text.
    if (n > 0x10FFFF || (n >= 0xD800 && n <= 0xDFFF))
      return undefined

    output.splice(i, 0, n)
    i++
  }

  return String.fromCodePoint(...output)
}

/**
 * Decodes a single DNS label. Labels without the ACE prefix are returned as-is.
 */
export function decodeIdnLabel(label: string): string {
  if (!label.toLowerCase().startsWith(ACE_PREFIX))
    return label

  const decoded = decodePunycode(label.slice(ACE_PREFIX.length))
  // An undecodable label is left alone: a name that merely looks like punycode is
  // still a legitimate DNS label.
  return decoded === undefined || decoded === '' ? label : decoded
}

/**
 * Decodes every punycode label of a domain or record name for display.
 * Returns the input unchanged when it holds no internationalized label, so
 * ASCII names never take a detour.
 */
export function toUnicodeDomain(value: string | undefined | null): string {
  if (!value)
    return ''
  if (!value.toLowerCase().includes(ACE_PREFIX))
    return value

  return value.split('.').map(decodeIdnLabel).join('.')
}

/**
 * True when displaying the domain differs from the stored ASCII form, which is
 * when showing the original alongside it is worth the space.
 */
export function isIdnDomain(value: string | undefined | null): boolean {
  return !!value && toUnicodeDomain(value) !== value
}
