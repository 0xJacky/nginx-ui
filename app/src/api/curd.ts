import http from '@/lib/http'

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
  pagination: Pagination
}

class Curd<T> {
  protected readonly baseUrl: string
  protected readonly plural: string

  get_list = this._get_list.bind(this)
  get = this._get.bind(this)
  save = this._save.bind(this)
  destroy = this._destroy.bind(this)
  recover = this._recover.bind(this)
  update_order = this._update_order.bind(this)

  constructor(baseUrl: string, plural: string | null = null) {
    this.baseUrl = baseUrl
    this.plural = plural ?? `${this.baseUrl}s`
  }

  // eslint-disable-next-line ts/no-explicit-any
  _get_list(params: any = null): Promise<GetListResponse<T>> {
    return http.get(this.plural, { params })
  }

  // eslint-disable-next-line ts/no-explicit-any
  _get(id: any = null, params: any = {}): Promise<T> {
    return http.get(this.baseUrl + (id ? `/${id}` : ''), { params })
  }

  // eslint-disable-next-line ts/no-explicit-any
  _save(id: any = null, data: any = undefined, config: any = undefined): Promise<T> {
    return http.post(this.baseUrl + (id ? `/${id}` : ''), data, config)
  }

  // eslint-disable-next-line ts/no-explicit-any
  _destroy(id: any = null, params: any = {}) {
    return http.delete(`${this.baseUrl}/${id}`, { params })
  }

  // eslint-disable-next-line ts/no-explicit-any
  _recover(id: any = null) {
    return http.patch(`${this.baseUrl}/${id}`)
  }

  _update_order(data: {
    target_id: number
    direction: number
    affected_ids: number[]
  }) {
    return http.post(`${this.plural}/order`, data)
  }
}

export default Curd
