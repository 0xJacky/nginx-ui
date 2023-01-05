import Curd from '@/api/curd'
import http from '@/lib/http'

class Template extends Curd {
    get_config_list() {
        return http.get('template/configs')
    }

    get_block_list() {
        return http.get('template/blocks')
    }

    get_config(name: string) {
        return http.get('template/config/' + name)
    }

    get_block(name: string) {
        return http.get('template/block/' + name)
    }

}

const template = new Template('/template')

export default template
