import http from '@/lib/http'

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

    get_template() {
        return http.get('template')
    },

    cert_info(domain) {
        return http.get('cert/' + domain + '/info')
    },

    add_auto_cert(domain) {
        return http.post('cert/' + domain)
    },

    remove_auto_cert(domain) {
        return http.delete('cert/' + domain)
    }
}

export default domain
