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

export interface IGetListResponse<T> {
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

  constructor(baseUrl: string, plural: string | null = null) {
    this.baseUrl = baseUrl
    this.plural = plural ?? `${this.baseUrl}s`
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  _get_list(params: any = null): Promise<IGetListResponse<T>> {
    return http.get(this.plural, { params })
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  _get(id: any = null): Promise<T> {
    return http.get(this.baseUrl + (id ? `/${id}` : ''))
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  _save(id: any = null, data: any, config: any = undefined): Promise<T> {
    return http.post(this.baseUrl + (id ? `/${id}` : ''), data, config)
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  _destroy(id: any = null) {
    return http.delete(`${this.baseUrl}/${id}`)
  }
}

export default Curd
