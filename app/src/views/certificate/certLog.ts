import { T } from '@/language'

const logLevelLabels: Record<string, string> = {
  INFO: 'Info',
  WARN: 'Warning',
  ERROR: 'Error',
  DEBUG: 'Debug',
}

// Keep these literals in source so gettext extraction includes structured log
// message keys that arrive dynamically from ACME libraries.
function structuredLogMessageI18nHints() {
  return [
    $gettext('Trying renewal.'),
    $gettext('Obtaining bundled SAN certificate.'),
    $gettext('Use solver.'),
    $gettext('http01: Trying to solve HTTP-01.'),
    $gettext('The server validated our request.'),
    $gettext('Validations succeeded; requesting certificates.'),
    $gettext('Waiting for certificates.'),
    $gettext('Server responded with a certificate.'),
  ]
}
void structuredLogMessageI18nHints

function structuredLogFieldI18nHints() {
  return [
    $gettext('domains'),
    $gettext('domain'),
    $gettext('type'),
    $gettext('hoursRemaining'),
    $gettext('timeout'),
    $gettext('interval'),
  ]
}
void structuredLogFieldI18nHints

function localizeStructuredLevelValue(raw: string) {
  return raw.replace(/(^|\s)(level|等级|層級)=([A-Z]+)/g, (_, prefix: string, key: string, level: string) => {
    const mapped = logLevelLabels[level] || level
    return `${prefix}${key}=${$gettext(mapped)}`
  })
}

function stripTimeKey(raw: string) {
  return raw.replace(/^(time|时间|時間)=(\S+)/, '$2')
}

function applyKeywordLineBreaks(raw: string) {
  return raw.replace(/\s+(消息|msg|訊息|域名列表|domains|網域列表|域名|domain|網域|type|timeout|interval|hoursRemaining)=/g, '\n$1=')
}

function applyAuxiliaryFieldFormatting(raw: string) {
  let localized = raw.replace(/^(domain|type|hoursRemaining|timeout|interval)=([^\n]*)$/gm, (_, key: string, value: string) => {
    return `${$gettext(key)}:${value}`
  })

  localized = localized.replace(/(domains)=("([^"]*)"|(\S+))/g, (_, key: string, full: string, quoted: string | undefined, plain: string | undefined) => {
    const value = (quoted ?? plain ?? '').trim()
    const domains = value
      .split(/[\s,，;；]+/)
      .map(item => item.trim())
      .filter(Boolean)
    const label = $gettext(key)

    if (domains.length <= 1)
      return `${label}:${full}`

    return `${label}:\n${domains.map(domain => `- ${domain}`).join('\n')}`
  })

  return localized
}

function applyNginxUILineBreaks(raw: string) {
  return raw
    .replace(/，邮箱：/g, '\n邮箱：')
    .replace(/,\s*Email:/g, '\nEmail:')
    .replace(/，CA 目录：/g, '\nCA 目录：')
    .replace(/,\s*CA Dir:/g, '\nCA Dir:')
}

function stripMessageWrapper(raw: string) {
  return raw
    .replace(/^(msg|消息|訊息)="([^"]*)"$/gm, '$2')
    .replace(/^(msg|消息|訊息)=(.+)$/gm, '$2')
}

function localizeStructuredLogLine(raw: string) {
  const translatedWhole = $gettext(raw)
  if (translatedWhole !== raw)
    return translatedWhole

  let localized = localizeStructuredLevelValue(raw)
  localized = stripTimeKey(localized)

  const match = raw.match(/msg="([^"]+)"/)
  if (!match)
    return localized

  const originalMessage = match[1]
  const translatedMessage = $gettext(originalMessage)

  if (translatedMessage !== originalMessage) {
    localized = localized.replace(`msg="${originalMessage}"`, `msg="${translatedMessage}"`)
    localized = localized.replace(`消息="${originalMessage}"`, `消息="${translatedMessage}"`)
    localized = localized.replace(`訊息="${originalMessage}"`, `訊息="${translatedMessage}"`)
  }

  localized = applyKeywordLineBreaks(localized)
  localized = applyAuxiliaryFieldFormatting(localized)
  return stripMessageWrapper(localized)
}

function renderLocalizedLogMessage(raw: string) {
  const matches = raw.match(/\[Nginx UI\] (.*)/)
  if (matches?.[1])
    return applyNginxUILineBreaks(raw.replaceAll(matches[1], $gettext(matches[1])))

  return localizeStructuredLogLine(raw)
}

function localizeLine(line: string) {
  try {
    return renderLocalizedLogMessage(T(JSON.parse(line)))
  }
  catch {
    return renderLocalizedLogMessage(line)
  }
}

/** Localizes the stored issuance log for display. */
export function renderCertificateLog(raw?: string): string {
  if (!raw)
    return ''

  return raw.split('\n')
    .map(localizeLine)
    .map(line => line.startsWith('[Nginx UI]') ? `${line}\n` : line)
    .join('\n')
}

/** The last `count` non-empty lines of the localized log. */
export function lastCertificateLogLines(raw: string | undefined, count: number): string[] {
  if (!raw)
    return []

  return raw.split('\n')
    .filter(line => line.trim())
    .map(localizeLine)
    .flatMap(line => line.split('\n'))
    .filter(line => line.trim())
    .slice(-count)
}
