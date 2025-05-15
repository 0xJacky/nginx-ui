import { http } from '@uozi-admin/request'

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

class Curd<T> {
  protected readonly baseUrl: string

  get_list = this._get_list.bind(this)
  get = this._get.bind(this)
  save = this._save.bind(this)
  import = this._import.bind(this)
  import_check = this._import_check.bind(this)
  destroy = this._destroy.bind(this)
  recover = this._recover.bind(this)
  update_order = this._update_order.bind(this)
  batch_save = this._batch_save.bind(this)
  batch_destroy = this._batch_destroy.bind(this)
  batch_recover = this._batch_recover.bind(this)

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl
  }

  // eslint-disable-next-line ts/no-explicit-any
  _get_list(params: any = null): Promise<GetListResponse<T>> {
    return http.get(this.baseUrl, { params })
  }

  // eslint-disable-next-line ts/no-explicit-any
  _get(id: any = null, params: any = {}): Promise<T> {
    return http.get(this.baseUrl + (id ? `/${encodeURIComponent(id)}` : ''), { params })
  }

  // eslint-disable-next-line ts/no-explicit-any
  _save(id: any = null, data: any = {}, config: any = undefined): Promise<T> {
    return http.post(this.baseUrl + (id ? `/${encodeURIComponent(id)}` : ''), data, config)
  }

  // eslint-disable-next-line ts/no-explicit-any
  _import_check(formData: FormData, config: any = {}): Promise<T> {
    return http.post(`${this.baseUrl}/import_check`, formData, {
      headers: {
        'Content-Type': 'multipart/form-data;charset=UTF-8',
      },
      ...config,
    })
  }

  // eslint-disable-next-line ts/no-explicit-any
  _import(data: any, config: any = {}): Promise<T> {
    return http.post(`${this.baseUrl}/import`, data, config)
  }

  // eslint-disable-next-line ts/no-explicit-any
  _destroy(id: any = null, params: any = {}) {
    return http.delete(`${this.baseUrl}/${encodeURIComponent(id)}`, { params })
  }

  // eslint-disable-next-line ts/no-explicit-any
  _recover(id: any = null) {
    return http.patch(`${this.baseUrl}/${encodeURIComponent(id)}`)
  }

  _update_order(data: { target_id: number, direction: number, affected_ids: number[] }) {
    return http.post(`${this.baseUrl}/order`, data)
  }

  // eslint-disable-next-line ts/no-explicit-any
  _batch_save(ids: any, data: any) {
    return http.put(this.baseUrl, {
      ids,
      data,
    })
  }

  // eslint-disable-next-line ts/no-explicit-any
  _batch_destroy(ids?: (string | number)[], params: any = {}) {
    return http.delete(this.baseUrl, {
      params,
      data: {
        ids,
      },
    })
  }

  _batch_recover(ids?: (string | number)[]) {
    return http.patch(this.baseUrl, {
      data: {
        ids,
      },
    })
  }
}

export default Curd
