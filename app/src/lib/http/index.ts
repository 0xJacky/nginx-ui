import axios, {AxiosRequestConfig} from 'axios'
import {useSettingsStore, useUserStore} from '@/pinia'
import {storeToRefs} from 'pinia'
import NProgress from 'nprogress'
import 'nprogress/nprogress.css'

import router from '@/routes'

const user = useUserStore()
const settings = useSettingsStore()
const {token} = storeToRefs(user)

let instance = axios.create({
  baseURL: import.meta.env.VITE_API_ROOT,
  timeout: 50000,
  headers: {'Content-Type': 'application/json'},
  transformRequest: [function (data, headers) {
    if (!(headers) || headers['Content-Type'] === 'multipart/form-data;charset=UTF-8') {
      return data
    } else {
      headers['Content-Type'] = 'application/json'
    }
    return JSON.stringify(data)
  }]
})


instance.interceptors.request.use(
  config => {
    NProgress.start()
    if (token) {
      (config.headers as any).Authorization = token.value
    }
    if (settings.environment.id) {
      (config.headers as any)['X-Node-ID'] = settings.environment.id
    }
    return config
  },
  err => {
    return Promise.reject(err)
  }
)

instance.interceptors.response.use(
  response => {
    NProgress.done()
    return Promise.resolve(response.data)
  },
  async error => {
    NProgress.done()
    switch (error.response.status) {
      case 401:
      case 403:
        user.logout()
        await router.push('/login')
        break
    }
    return Promise.reject(error.response.data)
  }
)

const http = {
  get(url: string, config: AxiosRequestConfig = {}) {
    return instance.get<any, any>(url, config)
  },
  post(url: string, data: any = undefined, config: AxiosRequestConfig = {}) {
    return instance.post<any, any>(url, data, config)
  },
  put(url: string, data: any = undefined, config: AxiosRequestConfig = {}) {
    return instance.put<any, any>(url, data, config)
  },
  delete(url: string, config: AxiosRequestConfig = {}) {
    return instance.delete<any, any>(url, config)
  }
}


export default http
