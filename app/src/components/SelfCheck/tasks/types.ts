import type { ReportStatusType, TaskReport as SelfCheckTaskReport } from '@/api/self_check'

export interface TaskDefinition extends Pick<SelfCheckTaskReport, 'key' | 'fixable' | 'err'> {
  name: () => string
  description: () => string
}

export interface FrontendTask extends TaskDefinition {
  check: () => Promise<ReportStatusType>
}

export interface TaskReport extends TaskDefinition {
  status: ReportStatusType
}
