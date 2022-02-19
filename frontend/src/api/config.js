import http from '@/lib/http'

const base_url = '/config'

const config = {
    get_list(params) {
        return http.get(base_url + 's', {params: params})
    },

    get(id) {
        return http.get(base_url + '/' + id)
    },

    save(id = null, data) {
        return http.post(base_url + (id ? '/' + id : ''), data)
    },

    destroy(id) {
        return http.delete(base_url + '/' + id)
    }
}

export default config
