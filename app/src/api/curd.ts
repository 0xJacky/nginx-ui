export interface ModelBase {
  id: number
  created_at: string
  updated_at: string
}

export interface Pagination {
  total: number
  per_page: number
  current_page: number
  total_pages: number
}

export interface GetListResponse<T> {
  data: T[]
  pagination?: Pagination
}

export interface UpdateOrderRequest {
  target_id: number
  direction: number
  affected_ids: number[]
}
