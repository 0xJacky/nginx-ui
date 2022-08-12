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
    
    add_auto_cert(domain: string) {
        return http.post('cert/' + domain)
    }

    remove_auto_cert(domain: string) {
        return http.delete('cert/' + domain)
    }
}

const domain = new Domain('/domain')

export default domain
