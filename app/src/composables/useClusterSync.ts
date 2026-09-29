import type { SyncSkipReason, SyncSummary } from '@/api/cluster_sync'

/** Maximum number of failures listed in a single notification. */
const maxReportedFailures = 5

function skipReasonText(reason: SyncSkipReason): string {
  switch (reason) {
    case 'unsupported_type':
      return $gettext('unsupported file type')
    case 'too_large':
      return $gettext('larger than 5 MB')
    case 'not_text':
      return $gettext('not a text file')
    case 'unreadable':
      return $gettext('unreadable')
    case 'entry_config':
      return $gettext('node specific entry configuration')
    default:
      return reason
  }
}

/**
 * Reports the outcome of a cluster synchronization run.
 *
 * Successful runs collapse into one short toast, failures are listed so the user
 * knows which node rejected which item.
 */
export function useClusterSync() {
  const { message, notification } = useGlobalApp()

  function skippedDescription(summary: SyncSummary): string {
    const skipped = summary.skipped ?? []
    const lines = skipped
      .slice(0, maxReportedFailures)
      .map(item => `${item.path} (${skipReasonText(item.reason)})`)
    if (skipped.length > maxReportedFailures) {
      lines.push($gettext('and %{count} more', { count: (skipped.length - maxReportedFailures).toString() }))
    }

    return lines.join('\n')
  }

  function existingDescription(summary: SyncSummary): string {
    return summary.results
      .filter(item => (item.skipped_existing ?? 0) > 0)
      .slice(0, maxReportedFailures)
      .map(item => {
        const paths = item.skipped_paths ?? []
        const detail = paths.length > 0 ? paths.slice(0, maxReportedFailures).join(', ') : item.name
        return $gettext('%{node}: %{count} existing files were not overwritten (%{detail})', {
          node: item.node,
          count: (item.skipped_existing ?? 0).toString(),
          detail,
        })
      })
      .join('\n')
  }

  function report(summary: SyncSummary) {
    const skippedCount = summary?.skipped?.length ?? 0
    const existing = summary ? existingDescription(summary) : ''

    if (!summary || summary.total === 0) {
      if (skippedCount > 0) {
        notification.warning({
          title: $gettext('Nothing was synchronized, %{count} files were skipped', { count: skippedCount.toString() }),
          description: skippedDescription(summary),
          duration: 10,
        })
        return
      }
      message.info($gettext('There is nothing to synchronize'))
      return
    }

    if (summary.failed === 0) {
      if (skippedCount > 0 || existing) {
        const parts = [
          existing,
          skippedCount > 0 ? `${$gettext('Skipped files:')}\n${skippedDescription(summary)}` : '',
        ].filter(Boolean)
        notification.warning({
          title: $gettext('Synchronized %{count} items with some files skipped', {
            count: summary.succeeded.toString(),
          }),
          description: parts.join('\n\n'),
          duration: 10,
        })
        return
      }
      message.success($gettext('Synchronized %{count} items successfully', { count: summary.succeeded.toString() }))
      return
    }

    const failures = summary.results.filter(item => !item.success)
    const failureText = failures
      .slice(0, maxReportedFailures)
      .map(item => `${item.node}: ${item.name} - ${item.error}`)
      .join('\n')
    const description = [
      failureText,
      existing,
      skippedCount > 0 ? `${$gettext('Skipped files:')}\n${skippedDescription(summary)}` : '',
    ].filter(Boolean).join('\n\n')

    notification.error({
      title: $gettext('Synchronization finished with %{count} failures', { count: summary.failed.toString() }),
      description,
      duration: 10,
    })
  }

  return { report }
}
