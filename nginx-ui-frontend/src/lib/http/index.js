import axios from 'axios'

/* 创建 axios 实例 */
let http = axios.create({
    baseURL: process.env.VUE_APP_API_ROOT,
    timeout: 50000,
    headers: {'Content-Type': 'application/json'},
    transformRequest: [function (data, headers) {
        if (headers['Content-Type'] === 'multipart/form-data;charset=UTF-8') {
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
        return config
    },
    err => {
        return Promise.reject(err)
    }
)

/* response 拦截器 */
http.interceptors.response.use(
    response => {
        return Promise.resolve(response.data)
    },
    async error => {
        console.log(error)
        return Promise.reject(error.response.data)
    }
)

export default http
