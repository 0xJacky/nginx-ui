import type { TaskDefinition, TaskReport, TaskStatus } from './types'
import selfCheck from '@/api/self_check'
import backendTasks from './backend'
import frontendTasks from './frontend'

// Combine all tasks
const allTasks: Record<string, TaskDefinition> = {
  ...backendTasks,
  ...frontendTasks,
}

// Task manager
export const taskManager = {
  // Get all task definitions
  getAllTasks() {
    return allTasks
  },

  // Execute all self-checks
  async runAllChecks(): Promise<TaskReport[]> {
    // Execute backend checks
    const backendReports = await selfCheck.run()

    // Convert backend reports to include status field
    const convertedBackendReports = backendReports.map(report => {
      const status: TaskStatus = report.err ? 'error' : 'success'
      return {
        ...report,
        type: 'backend' as const,
        status,
      }
    })

    // Execute frontend checks - they now directly return TaskReport objects
    const frontendReports = await Promise.all(
      Object.entries(frontendTasks).map(async ([key, task]) => {
        try {
          return await task.check()
        }
        catch (err) {
          // Fallback error handling in case a task throws instead of returning a report
          return {
            name: key,
            type: 'frontend' as const,
            status: 'error' as const,
            message: 'Task execution failed',
            err: err instanceof Error ? err : new Error(String(err)),
          }
        }
      }),
    )

    // Merge results
    return [
      ...convertedBackendReports,
      ...frontendReports,
    ]
  },

  // Fix task
  async fixTask(taskName: string): Promise<boolean> {
    // Backend task
    if (taskName in backendTasks) {
      await selfCheck.fix(taskName)
      return true
    }

    // Frontend task
    if (taskName in frontendTasks) {
      const task = frontendTasks[taskName]
      if (task.fix) {
        return await task.fix()
      }
      return false
    }

    return false
  },

  // Get task definition
  getTask(taskName: string) {
    return allTasks[taskName]
  },
}

export default allTasks

export * from './types'
