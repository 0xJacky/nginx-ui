import type { CosyError } from '@/lib/http'

export type TaskStatus = 'success' | 'warning' | 'error'

export interface TaskDefinition {
  name: () => string
  description: () => string
  type: 'backend' | 'frontend'
}

export interface BackendTask extends TaskDefinition {
  type: 'backend'
}

export interface FrontendTask extends TaskDefinition {
  type: 'frontend'
  check: () => Promise<TaskReport>
  fix?: () => Promise<boolean>
}

export interface TaskReport {
  name: string
  status: TaskStatus
  message?: string
  err?: CosyError | Error
  type: 'backend' | 'frontend'
}
