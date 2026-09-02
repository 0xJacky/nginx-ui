import type { SyncSummary } from '@/api/cluster_sync'

/** Maximum number of failures listed in a single notification. */
const maxReportedFailures = 5

/**
 * Reports the outcome of a cluster synchronization run.
 *
 * Successful runs collapse into one short toast, failures are listed so the user
 * knows which node rejected which item.
 */
export function useClusterSync() {
  const { message, notification } = useGlobalApp()

  function report(summary: SyncSummary) {
    if (!summary || summary.total === 0) {
      message.info($gettext('There is nothing to synchronize'))
      return
    }

    if (summary.failed === 0) {
      message.success($gettext('Synchronized %{count} items successfully', { count: summary.succeeded.toString() }))
      return
    }

    const failures = summary.results.filter(item => !item.success)
    const description = failures
      .slice(0, maxReportedFailures)
      .map(item => `${item.node}: ${item.name} - ${item.error}`)
      .join('\n')

    notification.error({
      message: $gettext('Synchronization finished with %{count} failures', { count: summary.failed.toString() }),
      description,
      duration: 10,
    })
  }

  return { report }
}
