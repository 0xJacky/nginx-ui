import http from '@/lib/http'
import {useUserStore} from '@/pinia'

const user = useUserStore()
const {login, logout} = user

const auth = {
  async login(name: string, password: string) {
    return http.post('/login', {
      name: name,
      password: password
    }).then(r => {
      login(r.token)
    })
  },
  async casdoorLogin(code: string, state: string) {
    await http.post('/casdoor_callback', {
      code: code,
      state: state
    })
      .then((r) => {
        login(r.token)
      })
  },
  logout() {
    return http.delete('/logout').then(async () => {
      logout()
    })
  }
}

export default auth
