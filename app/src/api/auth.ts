import http from '@/lib/http'
import { useUserStore } from '@/pinia'

const { login, logout } = useUserStore()

export interface AuthResponse {
  message: string
  token: string
}

const auth = {
  async login(name: string, password: string) {
    return http.post('/login', {
      name,
      password,
    }).then((r: AuthResponse) => {
      login(r.token)
    })
  },
  async casdoor_login(code?: string, state?: string) {
    await http.post('/casdoor_callback', {
      code,
      state,
    })
      .then((r: AuthResponse) => {
        login(r.token)
      })
  },
  async logout() {
    return http.delete('/logout').then(async () => {
      logout()
    })
  },
  async get_casdoor_uri(): Promise<{ uri: string }> {
    return http.get('/casdoor_uri')
  },
}

export default auth
