import axios, {AxiosRequestConfig} from 'axios'
import {useUserStore} from '@/pinia'
import {storeToRefs} from 'pinia'

const user = useUserStore()

const {token} = storeToRefs(user)

declare module 'axios' {
    export interface AxiosResponse<T = any> extends Promise<T> {
    }
}

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
    }],
})


instance.interceptors.request.use(
    config => {
        if (token) {
            (config.headers || {}).Authorization = token.value
        }
        return config
    },
    err => {
        return Promise.reject(err)
    }
)


instance.interceptors.response.use(
    response => {
        return Promise.resolve(response.data)
    },
    async error => {
        switch (error.response.status) {
            case 401:
            case 403:
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
