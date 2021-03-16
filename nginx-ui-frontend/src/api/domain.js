import http from "@/lib/http"

const base_url = '/domain'

const domain = {
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
    },

    enable(name) {
        return http.post(base_url + '/' + name + '/enable')
    },

    disable(name) {
        return http.post(base_url + '/' + name + '/disable')
    },

    get_template(name) {
        return http.get('template/' + name)
    }
}

export default domain
