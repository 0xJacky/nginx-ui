import http from '@/lib/http'

class Curd {
  protected readonly baseUrl: string
  protected readonly plural: string

  get_list = this._get_list.bind(this)
  get = this._get.bind(this)
  save = this._save.bind(this)
  destroy = this._destroy.bind(this)

  constructor(baseUrl: string, plural: string | null = null) {
    this.baseUrl = baseUrl
    this.plural = plural ?? this.baseUrl + 's'
  }

  _get_list(params: any = null) {
    return http.get(this.plural, {params: params})
  }

  _get(id: any = null) {
    return http.get(this.baseUrl + (id ? '/' + id : ''))
  }

  _save(id: any = null, data: any, config: any = undefined) {
    return http.post(this.baseUrl + (id ? '/' + id : ''), data, config)
  }

  _destroy(id: any = null) {
    return http.delete(this.baseUrl + '/' + id)
  }
}

export default Curd
