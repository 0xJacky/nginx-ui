import Curd from '@/api/curd'
import http from '@/lib/http'
import {AxiosRequestConfig} from "axios/index";

class Domain extends Curd {
    enable(name: string, config: AxiosRequestConfig) {
        return http.post(this.baseUrl + '/' + name + '/enable', undefined, config)
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

    advance_mode(name: string, data: any) {
        return http.post(this.baseUrl + '/' + name + '/advance', data)
    }
}

const domain = new Domain('/domain')

export default domain
