import axios from 'axios'
import {useUserStore} from "@/pinia/user"
import {storeToRefs} from "pinia";

const user = useUserStore()

const {token} = storeToRefs(user)

/* 创建 axios 实例 */
let http = axios.create({
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

/* http request 拦截器 */
http.interceptors.request.use(
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

/* response 拦截器 */
http.interceptors.response.use(
    response =>{
        return Promise.resolve(response)
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

export default http
