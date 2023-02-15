import Curd from '@/api/curd'
import http from '@/lib/http'

class Domain extends Curd {
    enable(name: string) {
        return http.post(this.baseUrl + '/' + name + '/enable')
    }

    disable(name: string) {
        return http.post(this.baseUrl + '/' + name + '/disable')
    }

    get_template() {
        return http.get('template')
    }

    add_auto_cert(domain: string, data: any) {
        return http.post('auto_cert/' + domain, data)
    }

    remove_auto_cert(domain: string) {
        return http.delete('auto_cert/' + domain)
    }

    duplicate(name: string, data: any) {
        return http.post(this.baseUrl + '/' + name + '/duplicate', data)
    }
}

const domain = new Domain('/domain')

export default domain
