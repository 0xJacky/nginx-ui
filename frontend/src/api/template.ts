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

  build_block(name: string, data: any) {
    return http.post('template/block/' + name, data)
  }

}

const template = new Template('/template')

export default template
