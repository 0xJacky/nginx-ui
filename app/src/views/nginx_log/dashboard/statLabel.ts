// The log parser leaves the browser and OS empty when it does not recognize a
// user agent (curl, scripts, "-"), and the dashboard facets report those rows
// as an empty term. Label that term instead of rendering a blank cell.
export function statLabel(value: string | undefined | null) {
  return value?.trim()
    ? value
    : $gettext('Unknown')
}
