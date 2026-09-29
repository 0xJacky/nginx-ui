/**
 * Whether the log page draws the file picker and the view switch itself, in a
 * header row above the view.
 *
 * The built-in structured viewer puts them into its own toolbar instead, so
 * only that viewer opts out. A view a plugin provides has no such toolbar and
 * always gets the header, whatever its key.
 */
export function showsHostViewChrome(effectiveView: string, pluginViewKey: string | undefined): boolean {
  return pluginViewKey !== undefined || effectiveView !== 'structured'
}
