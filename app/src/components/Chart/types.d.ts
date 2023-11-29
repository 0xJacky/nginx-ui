import type {Usage} from '@/api/analytic'

export interface Series {
  name: string
  data: Usage[]
}
